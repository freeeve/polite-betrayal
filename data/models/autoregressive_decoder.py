"""Autoregressive order decoder for Diplomacy policy network.

Replaces independent per-unit logit heads with a sequential decoder that
generates orders unit-by-unit, conditioning each order on previously
generated orders. This allows the model to learn coordinated move+support
pairs and avoid assigning two units to the same province.

Architecture:
  1. Encoder: Shared GAT encoder from gnn.py (produces [B, 81, D] embeddings)
  2. Decoder: Small Transformer decoder (2 layers, 256-d) that generates
     orders sequentially:
       - Input at each step: unit embedding + order embedding of previous step
       - Cross-attention to full board embeddings
       - Output: distribution over 169-dim order vocabulary
  3. Training: Teacher forcing (feed ground truth previous orders)
  4. Inference: Autoregressive (feed own predictions as previous orders)

Unit ordering is deterministic: units sorted by province index (ascending).

ONNX strategy: Export encoder and single decoder step as separate models.
The autoregressive loop runs in Rust, calling the decoder step model
once per unit.
"""

import math

import torch
import torch.nn as nn
import torch.nn.functional as F

from gnn import DiplomacyPolicyNet, GATBlock


NUM_AREAS = 81
NUM_FEATURES = 47
NUM_POWERS = 7
ORDER_VOCAB_SIZE = 169
MAX_UNITS = 17


class OrderEmbedding(nn.Module):
    """Embeds a 169-dim order vector into a dense representation.

    The order vector is multi-hot: [7 order types, 81 source areas, 81 dest areas].
    We embed each component separately and sum them, which preserves the
    compositional structure while being more parameter-efficient than a
    single linear projection.
    """

    def __init__(self, hidden_dim: int):
        super().__init__()
        self.type_embed = nn.Embedding(7, hidden_dim)
        self.src_embed = nn.Embedding(NUM_AREAS, hidden_dim)
        self.dst_embed = nn.Embedding(NUM_AREAS, hidden_dim)
        self.null_embed = nn.Parameter(torch.zeros(hidden_dim))
        nn.init.normal_(self.null_embed, std=0.02)

    def forward(self, order_vec: torch.Tensor) -> torch.Tensor:
        """Embed an order vector.

        Args:
            order_vec: [B, 169] multi-hot order vector, or [B, S, 169] for sequences.

        Returns:
            [B, D] or [B, S, D] order embedding.
        """
        squeeze = False
        if order_vec.dim() == 2:
            order_vec = order_vec.unsqueeze(1)
            squeeze = True

        B, S, V = order_vec.shape

        # Extract component indices via argmax
        type_probs = order_vec[:, :, :7]  # [B, S, 7]
        src_probs = order_vec[:, :, 7:7 + NUM_AREAS]  # [B, S, 81]
        dst_probs = order_vec[:, :, 7 + NUM_AREAS:]  # [B, S, 81]

        type_idx = type_probs.argmax(dim=-1)  # [B, S]
        src_idx = src_probs.argmax(dim=-1)  # [B, S]
        dst_idx = dst_probs.argmax(dim=-1)  # [B, S]

        # Check for null (all-zero) orders
        has_order = order_vec.sum(dim=-1) > 0  # [B, S]

        emb = self.type_embed(type_idx) + self.src_embed(src_idx) + self.dst_embed(dst_idx)
        # Replace null orders with learned null embedding
        null = self.null_embed.unsqueeze(0).unsqueeze(0).expand(B, S, -1)
        emb = torch.where(has_order.unsqueeze(-1), emb, null)

        if squeeze:
            emb = emb.squeeze(1)
        return emb


class TransformerDecoderStep(nn.Module):
    """Single Transformer decoder layer with self-attention and cross-attention.

    Self-attention operates over the sequence of previously generated orders.
    Cross-attention attends to the full board embeddings from the encoder.
    """

    def __init__(self, hidden_dim: int, num_heads: int = 4, dropout: float = 0.1):
        super().__init__()
        self.self_attn = nn.MultiheadAttention(
            hidden_dim, num_heads, dropout=dropout, batch_first=True
        )
        self.cross_attn = nn.MultiheadAttention(
            hidden_dim, num_heads, dropout=dropout, batch_first=True
        )
        self.ffn = nn.Sequential(
            nn.Linear(hidden_dim, hidden_dim * 4),
            nn.GELU(),
            nn.Dropout(dropout),
            nn.Linear(hidden_dim * 4, hidden_dim),
            nn.Dropout(dropout),
        )
        self.norm1 = nn.LayerNorm(hidden_dim)
        self.norm2 = nn.LayerNorm(hidden_dim)
        self.norm3 = nn.LayerNorm(hidden_dim)

    def forward(
        self,
        x: torch.Tensor,
        memory: torch.Tensor,
        causal_mask: torch.Tensor | None = None,
    ) -> torch.Tensor:
        """Forward pass.

        Args:
            x: Decoder input sequence [B, S, D]
            memory: Encoder output (board embeddings) [B, N, D]
            causal_mask: Causal attention mask [S, S] (True = mask out)

        Returns:
            Updated sequence [B, S, D]
        """
        # Self-attention with causal mask
        residual = x
        x_attn, _ = self.self_attn(x, x, x, attn_mask=causal_mask)
        x = self.norm1(residual + x_attn)

        # Cross-attention to board embeddings
        residual = x
        x_cross, _ = self.cross_attn(x, memory, memory)
        x = self.norm2(residual + x_cross)

        # FFN
        residual = x
        x = self.norm3(residual + self.ffn(x))

        return x


class AutoregressiveDecoder(nn.Module):
    """Autoregressive order decoder.

    Generates orders sequentially, one per unit, conditioned on the board
    state and all previously generated orders.

    Architecture:
      - Position embedding for unit slot position
      - Order embedding for previous orders
      - 2 Transformer decoder layers with cross-attention to board
      - Output projection to 169-dim order vocabulary
    """

    def __init__(
        self,
        encoder_dim: int = 512,
        decoder_dim: int = 256,
        num_layers: int = 2,
        num_heads: int = 4,
        max_units: int = MAX_UNITS,
        order_vocab_size: int = ORDER_VOCAB_SIZE,
        dropout: float = 0.1,
    ):
        super().__init__()
        self.encoder_dim = encoder_dim
        self.decoder_dim = decoder_dim
        self.max_units = max_units
        self.order_vocab_size = order_vocab_size

        # Project encoder embeddings to decoder dimension
        self.memory_proj = nn.Linear(encoder_dim, decoder_dim)

        # Unit embedding: project unit's board embedding to decoder dim
        self.unit_proj = nn.Linear(encoder_dim, decoder_dim)

        # Positional embedding for unit sequence slot
        self.pos_embed = nn.Embedding(max_units, decoder_dim)

        # Order embedding for previously generated orders
        self.order_embed = OrderEmbedding(decoder_dim)

        # Start-of-sequence token (learned)
        self.sos_embed = nn.Parameter(torch.zeros(decoder_dim))
        nn.init.normal_(self.sos_embed, std=0.02)

        # Transformer decoder layers
        self.layers = nn.ModuleList([
            TransformerDecoderStep(decoder_dim, num_heads=num_heads, dropout=dropout)
            for _ in range(num_layers)
        ])

        # Output head
        self.output_head = nn.Sequential(
            nn.Linear(decoder_dim, decoder_dim),
            nn.GELU(),
            nn.LayerNorm(decoder_dim),
            nn.Linear(decoder_dim, order_vocab_size),
        )

    def _build_causal_mask(self, seq_len: int, device: torch.device) -> torch.Tensor:
        """Build a causal attention mask (upper triangular = True = masked)."""
        mask = torch.triu(torch.ones(seq_len, seq_len, device=device), diagonal=1)
        return mask.bool()

    def _build_decoder_input(
        self,
        board_embeddings: torch.Tensor,
        unit_indices: torch.Tensor,
        prev_orders: torch.Tensor | None,
    ) -> torch.Tensor:
        """Build the decoder input sequence.

        At position i, the input is:
          unit_embedding[i] + pos_embedding[i] + order_embedding[i-1]

        At position 0, order_embedding is the learned SOS token.

        Args:
            board_embeddings: [B, 81, encoder_dim] from the encoder
            unit_indices: [B, S] province indices of units (-1 for padding)
            prev_orders: [B, S, 169] previous order vectors (teacher forcing)
                         or None (inference, built incrementally)

        Returns:
            [B, S, decoder_dim] decoder input sequence
        """
        B, S = unit_indices.shape
        D = self.decoder_dim
        device = board_embeddings.device

        # Get unit embeddings from board
        safe_idx = unit_indices.clamp(min=0)  # [B, S]
        safe_idx_exp = safe_idx.unsqueeze(-1).expand(B, S, self.encoder_dim)
        unit_emb = torch.gather(board_embeddings, 1, safe_idx_exp)  # [B, S, encoder_dim]
        unit_emb = self.unit_proj(unit_emb)  # [B, S, decoder_dim]

        # Position embeddings
        positions = torch.arange(S, device=device).unsqueeze(0).expand(B, -1)
        pos_emb = self.pos_embed(positions)  # [B, S, decoder_dim]

        # Shifted order embeddings (order at position i-1 feeds into position i)
        if prev_orders is not None:
            # Teacher forcing: use ground truth previous orders
            # Shift right: prepend SOS, drop last
            order_emb = self.order_embed(prev_orders)  # [B, S, decoder_dim]
            sos = self.sos_embed.unsqueeze(0).unsqueeze(0).expand(B, 1, D)
            shifted_order_emb = torch.cat([sos, order_emb[:, :-1]], dim=1)  # [B, S, D]
        else:
            # Inference: no previous orders yet, all SOS
            shifted_order_emb = self.sos_embed.unsqueeze(0).unsqueeze(0).expand(B, S, D)

        decoder_input = unit_emb + pos_emb + shifted_order_emb
        return decoder_input

    def forward_teacher_forcing(
        self,
        board_embeddings: torch.Tensor,
        unit_indices: torch.Tensor,
        power_indices: torch.Tensor,
        target_orders: torch.Tensor,
    ) -> torch.Tensor:
        """Forward pass with teacher forcing (training mode).

        All positions are computed in parallel using causal masking.

        Args:
            board_embeddings: [B, 81, encoder_dim] from GAT encoder
            unit_indices: [B, max_units] province indices (-1 for padding)
            power_indices: [B] power index (unused here, kept for API compat)
            target_orders: [B, max_units, 169] ground truth order vectors

        Returns:
            Order logits [B, max_units, 169]
        """
        B, S = unit_indices.shape
        device = board_embeddings.device

        # Project board embeddings to decoder dimension
        memory = self.memory_proj(board_embeddings)  # [B, 81, decoder_dim]

        # Build decoder input with shifted target orders
        decoder_input = self._build_decoder_input(
            board_embeddings, unit_indices, target_orders
        )

        # Causal mask: position i can only attend to positions 0..i
        causal_mask = self._build_causal_mask(S, device)

        # Run through decoder layers
        x = decoder_input
        for layer in self.layers:
            x = layer(x, memory, causal_mask)

        # Project to order logits
        logits = self.output_head(x)  # [B, S, 169]
        return logits

    def forward_autoregressive(
        self,
        board_embeddings: torch.Tensor,
        unit_indices: torch.Tensor,
        power_indices: torch.Tensor,
        temperature: float = 1.0,
    ) -> tuple[torch.Tensor, torch.Tensor]:
        """Autoregressive inference (generates one order at a time).

        Args:
            board_embeddings: [B, 81, encoder_dim] from GAT encoder
            unit_indices: [B, max_units] province indices (-1 for padding)
            power_indices: [B] power index (unused)
            temperature: Sampling temperature (1.0 = no scaling)

        Returns:
            Tuple of:
              - generated_orders: [B, max_units, 169] generated order vectors
              - logits: [B, max_units, 169] raw logits at each step
        """
        B, S = unit_indices.shape
        device = board_embeddings.device

        memory = self.memory_proj(board_embeddings)  # [B, 81, decoder_dim]
        generated = torch.zeros(B, S, self.order_vocab_size, device=device)
        all_logits = torch.zeros(B, S, self.order_vocab_size, device=device)

        # Build full decoder input with SOS (will update order embeddings each step)
        for step in range(S):
            # Build decoder input up to current step
            decoder_input = self._build_decoder_input(
                board_embeddings, unit_indices[:, :step + 1],
                generated[:, :step + 1] if step > 0 else None,
            )

            # Causal mask for current prefix
            causal_mask = self._build_causal_mask(step + 1, device)

            x = decoder_input
            for layer in self.layers:
                x = layer(x, memory, causal_mask)

            # Take output at last position
            step_logits = self.output_head(x[:, -1])  # [B, 169]
            all_logits[:, step] = step_logits

            # Greedy decode: take argmax and convert to one-hot
            if temperature > 0:
                scaled = step_logits / temperature
            else:
                scaled = step_logits
            pred_idx = scaled.argmax(dim=-1)  # [B]
            one_hot = F.one_hot(pred_idx, self.order_vocab_size).float()
            generated[:, step] = one_hot

        return generated, all_logits

    def _decode_step(
        self,
        board_embeddings: torch.Tensor,
        memory: torch.Tensor,
        unit_indices: torch.Tensor,
        generated: torch.Tensor,
        step: int,
    ) -> torch.Tensor:
        """Run a single decoder step and return logits.

        Args:
            board_embeddings: [B, 81, encoder_dim]
            memory: [B, 81, decoder_dim] projected board embeddings
            unit_indices: [B, S] province indices up to current step+1
            generated: [B, S, 169] generated orders so far
            step: current step index (0-based)

        Returns:
            Logits [B, 169] for the current step
        """
        decoder_input = self._build_decoder_input(
            board_embeddings, unit_indices[:, :step + 1],
            generated[:, :step + 1] if step > 0 else None,
        )
        causal_mask = self._build_causal_mask(step + 1, board_embeddings.device)
        x = decoder_input
        for layer in self.layers:
            x = layer(x, memory, causal_mask)
        return self.output_head(x[:, -1])  # [B, 169]

    def _build_destination_mask(
        self,
        generated: torch.Tensor,
        step: int,
    ) -> torch.Tensor:
        """Build a mask of already-claimed destination provinces.

        Returns a [B, 169] bool tensor where True means the order index
        is forbidden (destination province already used by an earlier move).
        Only masks destination slots (indices 88..168), not type or source.
        """
        B = generated.shape[0]
        V = self.order_vocab_size
        mask = torch.zeros(B, V, dtype=torch.bool, device=generated.device)

        if step == 0:
            return mask

        # Collect destination provinces used by previous move orders
        # Order vocab: [0:7] types, [7:88] src, [88:169] dst
        # Type index 1 = move, 4 = retreat (orders that claim a destination)
        TYPE_MOVE = 1
        TYPE_RETREAT = 4
        DST_START = 7 + NUM_AREAS  # 88

        for s in range(step):
            order = generated[:, s]  # [B, 169]
            order_type = order[:, :7].argmax(dim=-1)  # [B]
            is_movement = (order_type == TYPE_MOVE) | (order_type == TYPE_RETREAT)
            dst_section = order[:, DST_START:]  # [B, 81]
            has_dst = dst_section.sum(dim=-1) > 0  # [B]
            claims_dst = is_movement & has_dst  # [B]

            # For each batch element that claims a destination, mask that dst
            dst_idx = dst_section.argmax(dim=-1)  # [B]
            for b in range(B):
                if claims_dst[b]:
                    mask[b, DST_START + dst_idx[b]] = True

        return mask

    def forward_beam_search(
        self,
        board_embeddings: torch.Tensor,
        unit_indices: torch.Tensor,
        power_indices: torch.Tensor,
        beam_width: int = 5,
        constrain_destinations: bool = True,
    ) -> tuple[torch.Tensor, torch.Tensor]:
        """Beam search inference over the unit sequence.

        Expands K beams at each step, keeping the top-K candidates by
        cumulative log-probability. Only batch_size=1 is supported for
        beam search (typical inference pattern).

        Args:
            board_embeddings: [1, 81, encoder_dim]
            unit_indices: [1, S] province indices (-1 for padding)
            power_indices: [1] power index
            beam_width: Number of beams to maintain
            constrain_destinations: If True, mask out destination provinces
                already claimed by a move in the same beam

        Returns:
            Tuple of:
              - best_orders: [1, S, 169] best beam's generated orders
              - all_candidates: [beam_width, S, 169] all beam candidates
        """
        assert board_embeddings.shape[0] == 1, "Beam search requires batch_size=1"
        S = unit_indices.shape[1]
        device = board_embeddings.device
        K = beam_width
        V = self.order_vocab_size

        # Find number of valid units (non-padding)
        valid_mask = unit_indices[0] >= 0
        n_valid = valid_mask.sum().item()
        if n_valid == 0:
            empty = torch.zeros(1, S, V, device=device)
            return empty, empty.expand(K, -1, -1)

        memory = self.memory_proj(board_embeddings)  # [1, 81, decoder_dim]

        # Expand to K beams
        beam_board = board_embeddings.expand(K, -1, -1)    # [K, 81, encoder_dim]
        beam_memory = memory.expand(K, -1, -1)             # [K, 81, decoder_dim]
        beam_units = unit_indices.expand(K, -1)             # [K, S]
        beam_generated = torch.zeros(K, S, V, device=device)
        beam_scores = torch.zeros(K, device=device)         # log-probs

        for step in range(n_valid):
            # Get logits for current step across all beams
            logits = self._decode_step(
                beam_board, beam_memory, beam_units, beam_generated, step
            )  # [K, V]
            log_probs = F.log_softmax(logits, dim=-1)  # [K, V]

            # Apply destination constraint
            if constrain_destinations:
                dst_mask = self._build_destination_mask(beam_generated, step)
                log_probs = log_probs.masked_fill(dst_mask, float("-inf"))

            if step == 0:
                # First step: all beams are identical, only expand from beam 0
                scores = log_probs[0]  # [V]
                topk_scores, topk_indices = scores.topk(K)
                beam_scores = topk_scores
                for k in range(K):
                    one_hot = F.one_hot(topk_indices[k], V).float()
                    beam_generated[k, step] = one_hot
            else:
                # Expand each beam by top-K tokens
                # Total candidates: K * V, keep top K
                expanded = beam_scores.unsqueeze(1) + log_probs  # [K, V]
                flat = expanded.reshape(-1)  # [K*V]
                topk_scores, topk_flat = flat.topk(K)

                beam_idx = topk_flat // V  # which beam
                token_idx = topk_flat % V  # which token

                # Rebuild beams
                new_generated = beam_generated[beam_idx].clone()
                for k in range(K):
                    one_hot = F.one_hot(token_idx[k], V).float()
                    new_generated[k, step] = one_hot

                beam_generated = new_generated
                beam_scores = topk_scores

        # Best beam is index 0 (highest score)
        best = beam_generated[0:1]  # [1, S, V]
        return best, beam_generated  # [1, S, V], [K, S, V]

    def forward_topk_sampling(
        self,
        board_embeddings: torch.Tensor,
        unit_indices: torch.Tensor,
        power_indices: torch.Tensor,
        num_samples: int = 10,
        temperature: float = 1.0,
        top_k: int = 20,
        constrain_destinations: bool = True,
    ) -> tuple[torch.Tensor, torch.Tensor]:
        """Top-K sampling for diverse candidate generation.

        Generates multiple order sets by sampling from the top-K tokens
        at each step, with temperature control.

        Args:
            board_embeddings: [1, 81, encoder_dim]
            unit_indices: [1, S] province indices (-1 for padding)
            power_indices: [1] power index
            num_samples: Number of diverse candidates to generate
            temperature: Sampling temperature (lower = more greedy)
            top_k: Number of top tokens to sample from at each step
            constrain_destinations: Mask already-claimed destination provinces

        Returns:
            Tuple of:
              - candidates: [num_samples, S, 169] generated order sets
              - scores: [num_samples] log-probability of each candidate
        """
        assert board_embeddings.shape[0] == 1, "Top-K sampling requires batch_size=1"
        S = unit_indices.shape[1]
        device = board_embeddings.device
        N = num_samples
        V = self.order_vocab_size

        valid_mask = unit_indices[0] >= 0
        n_valid = valid_mask.sum().item()
        if n_valid == 0:
            empty = torch.zeros(N, S, V, device=device)
            return empty, torch.zeros(N, device=device)

        memory = self.memory_proj(board_embeddings)

        # Expand for parallel sampling
        sample_board = board_embeddings.expand(N, -1, -1)
        sample_memory = memory.expand(N, -1, -1)
        sample_units = unit_indices.expand(N, -1)
        sample_generated = torch.zeros(N, S, V, device=device)
        sample_scores = torch.zeros(N, device=device)

        for step in range(n_valid):
            logits = self._decode_step(
                sample_board, sample_memory, sample_units, sample_generated, step
            )  # [N, V]

            # Apply destination constraint
            if constrain_destinations:
                dst_mask = self._build_destination_mask(sample_generated, step)
                logits = logits.masked_fill(dst_mask, float("-inf"))

            # Temperature scaling
            scaled = logits / max(temperature, 1e-8)

            # Top-K filtering
            if top_k > 0 and top_k < V:
                top_values, _ = scaled.topk(top_k, dim=-1)
                threshold = top_values[:, -1].unsqueeze(-1)
                scaled = scaled.masked_fill(scaled < threshold, float("-inf"))

            probs = F.softmax(scaled, dim=-1)
            sampled_idx = torch.multinomial(probs, 1).squeeze(-1)  # [N]

            log_probs = F.log_softmax(logits, dim=-1)
            step_log_probs = log_probs.gather(1, sampled_idx.unsqueeze(1)).squeeze(1)
            sample_scores += step_log_probs

            one_hot = F.one_hot(sampled_idx, V).float()
            sample_generated[:, step] = one_hot

        return sample_generated, sample_scores

    def forward_single_step(
        self,
        memory: torch.Tensor,
        board_embeddings: torch.Tensor,
        unit_indices: torch.Tensor,
        prev_order_embs: torch.Tensor,
        step: int,
    ) -> torch.Tensor:
        """Single decoder step for ONNX export.

        This is the minimal computation needed per unit during inference.
        The autoregressive loop runs in the host (Rust).

        Args:
            memory: [B, 81, decoder_dim] projected board embeddings
            board_embeddings: [B, 81, encoder_dim] raw board embeddings
            unit_indices: [B, 1] province index for current unit
            prev_order_embs: [B, step, decoder_dim] accumulated order embeddings
            step: Current step index (0-based)

        Returns:
            Order logits [B, 169]
        """
        B = memory.shape[0]
        D = self.decoder_dim
        device = memory.device

        # Current unit embedding
        safe_idx = unit_indices.clamp(min=0)
        safe_idx_exp = safe_idx.unsqueeze(-1).expand(B, 1, self.encoder_dim)
        unit_emb = torch.gather(board_embeddings, 1, safe_idx_exp)
        unit_emb = self.unit_proj(unit_emb)  # [B, 1, D]

        # Position embedding for current step
        pos = torch.tensor([step], device=device).unsqueeze(0).expand(B, 1)
        pos_emb = self.pos_embed(pos)  # [B, 1, D]

        # Order embedding: SOS for first step, or last generated order
        if step == 0 or prev_order_embs is None or prev_order_embs.shape[1] == 0:
            order_emb = self.sos_embed.unsqueeze(0).unsqueeze(0).expand(B, 1, D)
        else:
            order_emb = prev_order_embs[:, -1:, :]  # [B, 1, D]

        # Single-position input
        current_input = unit_emb + pos_emb + order_emb  # [B, 1, D]

        # Concatenate with previous decoder states for self-attention context
        if step > 0 and prev_order_embs is not None and prev_order_embs.shape[1] > 0:
            # Reconstruct previous inputs for self-attention
            # For simplicity in ONNX, we just use cross-attention at each step
            pass

        # Run through decoder layers (single position attends to memory)
        x = current_input
        for layer in self.layers:
            x = layer(x, memory, causal_mask=None)

        logits = self.output_head(x.squeeze(1))  # [B, 169]
        return logits

    def count_parameters(self) -> int:
        """Return total number of trainable parameters."""
        return sum(p.numel() for p in self.parameters() if p.requires_grad)


class DiplomacyAutoRegressivePolicyNet(nn.Module):
    """Full autoregressive policy network: GAT encoder + AR decoder.

    Combines the shared GAT encoder from the existing policy network
    with the new autoregressive decoder.
    """

    def __init__(
        self,
        num_areas: int = NUM_AREAS,
        num_features: int = NUM_FEATURES,
        hidden_dim: int = 512,
        num_gat_layers: int = 6,
        num_heads: int = 8,
        decoder_dim: int = 256,
        decoder_layers: int = 2,
        decoder_heads: int = 4,
        num_powers: int = NUM_POWERS,
        max_units: int = MAX_UNITS,
        order_vocab_size: int = ORDER_VOCAB_SIZE,
        dropout: float = 0.1,
    ):
        super().__init__()
        self.num_areas = num_areas
        self.hidden_dim = hidden_dim

        # Encoder (same architecture as DiplomacyPolicyNet)
        self.input_proj = nn.Sequential(
            nn.Linear(num_features, hidden_dim),
            nn.GELU(),
            nn.LayerNorm(hidden_dim),
        )
        self.power_embed = nn.Embedding(num_powers, hidden_dim)
        self.gat_blocks = nn.ModuleList([
            GATBlock(hidden_dim, num_heads=num_heads, dropout=dropout)
            for _ in range(num_gat_layers)
        ])

        # Autoregressive decoder
        self.decoder = AutoregressiveDecoder(
            encoder_dim=hidden_dim,
            decoder_dim=decoder_dim,
            num_layers=decoder_layers,
            num_heads=decoder_heads,
            max_units=max_units,
            order_vocab_size=order_vocab_size,
            dropout=dropout,
        )

    def encode(self, board: torch.Tensor, adj: torch.Tensor, power_indices: torch.Tensor) -> torch.Tensor:
        """Encode board state into province embeddings with power context.

        Args:
            board: [B, 81, 47] board state tensor
            adj: [81, 81] adjacency matrix
            power_indices: [B] active power index

        Returns:
            Province embeddings [B, 81, hidden_dim]
        """
        x = self.input_proj(board)
        for block in self.gat_blocks:
            x = block(x, adj)

        # Add power context
        power_emb = self.power_embed(power_indices)  # [B, D]
        x = x + power_emb.unsqueeze(1)  # [B, N, D]

        return x

    def forward(
        self,
        board: torch.Tensor,
        adj: torch.Tensor,
        unit_indices: torch.Tensor,
        power_indices: torch.Tensor,
        target_orders: torch.Tensor | None = None,
    ) -> torch.Tensor:
        """Full forward pass.

        During training (target_orders provided): uses teacher forcing.
        During inference (target_orders is None): uses autoregressive generation.

        Args:
            board: [B, 81, 47]
            adj: [81, 81]
            unit_indices: [B, max_units]
            power_indices: [B]
            target_orders: [B, max_units, 169] (training only)

        Returns:
            Order logits [B, max_units, 169]
        """
        embeddings = self.encode(board, adj, power_indices)

        if target_orders is not None:
            # Teacher forcing mode
            logits = self.decoder.forward_teacher_forcing(
                embeddings, unit_indices, power_indices, target_orders
            )
        else:
            # Autoregressive inference
            _, logits = self.decoder.forward_autoregressive(
                embeddings, unit_indices, power_indices
            )

        return logits

    def beam_search(
        self,
        board: torch.Tensor,
        adj: torch.Tensor,
        unit_indices: torch.Tensor,
        power_indices: torch.Tensor,
        beam_width: int = 5,
        constrain_destinations: bool = True,
    ) -> tuple[torch.Tensor, torch.Tensor]:
        """Run beam search inference.

        Args:
            board: [1, 81, 47]
            adj: [81, 81]
            unit_indices: [1, max_units]
            power_indices: [1]
            beam_width: Number of beams
            constrain_destinations: Mask duplicate destination provinces

        Returns:
            Tuple of (best_orders [1, S, 169], all_beams [K, S, 169])
        """
        embeddings = self.encode(board, adj, power_indices)
        return self.decoder.forward_beam_search(
            embeddings, unit_indices, power_indices,
            beam_width=beam_width,
            constrain_destinations=constrain_destinations,
        )

    def generate_candidates(
        self,
        board: torch.Tensor,
        adj: torch.Tensor,
        unit_indices: torch.Tensor,
        power_indices: torch.Tensor,
        num_candidates: int = 10,
        beam_width: int = 5,
        temperature: float = 1.0,
        top_k: int = 20,
        constrain_destinations: bool = True,
    ) -> tuple[torch.Tensor, torch.Tensor]:
        """Generate a diverse pool of candidate order sets.

        Combines beam search (for high-quality candidates) with top-K
        sampling (for diversity). Beam candidates come first, then
        sampled candidates fill the remainder. Duplicates are removed.

        Args:
            board: [1, 81, 47]
            adj: [81, 81]
            unit_indices: [1, max_units]
            power_indices: [1]
            num_candidates: Total candidates to return
            beam_width: Beams for beam search
            temperature: Sampling temperature
            top_k: Top-K filtering for sampling
            constrain_destinations: Mask duplicate destination provinces

        Returns:
            Tuple of:
              - candidates: [N, S, 169] order sets (N <= num_candidates)
              - scores: [N] log-probability scores
        """
        embeddings = self.encode(board, adj, power_indices)
        S = unit_indices.shape[1]
        V = self.decoder.order_vocab_size
        device = board.device

        # Phase 1: beam search candidates
        _, beam_candidates = self.decoder.forward_beam_search(
            embeddings, unit_indices, power_indices,
            beam_width=min(beam_width, num_candidates),
            constrain_destinations=constrain_destinations,
        )

        # Phase 2: sample additional candidates if needed
        n_remaining = num_candidates - beam_candidates.shape[0]
        if n_remaining > 0:
            sampled, sample_scores = self.decoder.forward_topk_sampling(
                embeddings, unit_indices, power_indices,
                num_samples=n_remaining * 2,  # oversample to account for dedup
                temperature=temperature,
                top_k=top_k,
                constrain_destinations=constrain_destinations,
            )
        else:
            sampled = torch.zeros(0, S, V, device=device)

        # Combine and deduplicate
        all_candidates = torch.cat([beam_candidates, sampled], dim=0)

        # Score all candidates by computing their log-probabilities
        # Use teacher forcing to get logits for each candidate
        all_scores = self._score_candidates(embeddings, unit_indices, power_indices, all_candidates)

        # Deduplicate by argmax signature
        seen = set()
        unique_idx = []
        for i in range(all_candidates.shape[0]):
            sig = tuple(all_candidates[i].argmax(dim=-1).tolist())
            if sig not in seen:
                seen.add(sig)
                unique_idx.append(i)
            if len(unique_idx) >= num_candidates:
                break

        if not unique_idx:
            unique_idx = [0]

        idx_tensor = torch.tensor(unique_idx, device=device)
        return all_candidates[idx_tensor], all_scores[idx_tensor]

    def _score_candidates(
        self,
        embeddings: torch.Tensor,
        unit_indices: torch.Tensor,
        power_indices: torch.Tensor,
        candidates: torch.Tensor,
    ) -> torch.Tensor:
        """Score candidate order sets using teacher forcing log-probabilities.

        Args:
            embeddings: [1, 81, hidden_dim] encoded board
            unit_indices: [1, S] province indices
            power_indices: [1] power index
            candidates: [N, S, 169] candidate order sets

        Returns:
            Scores [N] (sum of log-probs over valid steps)
        """
        N, S, V = candidates.shape
        device = candidates.device

        # Expand encoder outputs for all candidates
        emb_exp = embeddings.expand(N, -1, -1)
        units_exp = unit_indices.expand(N, -1)
        power_exp = power_indices.expand(N)

        # Teacher forcing gives us logits at each position
        logits = self.decoder.forward_teacher_forcing(
            emb_exp, units_exp, power_exp, candidates
        )  # [N, S, V]

        log_probs = F.log_softmax(logits, dim=-1)  # [N, S, V]
        target_idx = candidates.argmax(dim=-1)  # [N, S]

        # Gather log-prob of the chosen token at each step
        step_log_probs = log_probs.gather(2, target_idx.unsqueeze(-1)).squeeze(-1)  # [N, S]

        # Mask padding (unit_indices == -1)
        valid = (units_exp >= 0).float()
        scores = (step_log_probs * valid).sum(dim=-1)  # [N]
        return scores

    def load_encoder_from_policy(self, policy_net: DiplomacyPolicyNet):
        """Copy encoder weights from a trained independent policy network.

        Enables warm-starting the autoregressive model from a pretrained
        independent model (transfer learning for the encoder).
        """
        self.input_proj.load_state_dict(policy_net.input_proj.state_dict())
        self.gat_blocks.load_state_dict(policy_net.gat_blocks.state_dict())
        self.power_embed.load_state_dict(policy_net.power_embed.state_dict())

    def count_parameters(self) -> int:
        """Return total number of trainable parameters."""
        return sum(p.numel() for p in self.parameters() if p.requires_grad)

    def count_encoder_parameters(self) -> int:
        """Return number of parameters in the encoder only."""
        count = sum(p.numel() for p in self.input_proj.parameters() if p.requires_grad)
        count += sum(p.numel() for p in self.gat_blocks.parameters() if p.requires_grad)
        count += sum(p.numel() for p in self.power_embed.parameters() if p.requires_grad)
        return count

    def count_decoder_parameters(self) -> int:
        """Return number of parameters in the decoder only."""
        return self.decoder.count_parameters()
