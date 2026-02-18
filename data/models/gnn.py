"""Graph Attention Network (GAT) policy model for Diplomacy move prediction.

Architecture:
  1. Linear input projection: 47 -> 256 per province node
  2. 3x GAT message-passing layers with residual connections + LayerNorm
  3. Per-unit order decoder: attention over province embeddings -> order logits

The model predicts a probability distribution over ~169-dim order vectors
for each unit belonging to the active power.

No torch_geometric dependency -- GAT layers are implemented from scratch.
"""

import math

import torch
import torch.nn as nn
import torch.nn.functional as F


class GATLayer(nn.Module):
    """Single-head Graph Attention layer (Velickovic et al., 2018).

    Computes attention-weighted message passing over a graph defined
    by an adjacency matrix. Supports multi-head attention.
    """

    def __init__(self, in_dim: int, out_dim: int, num_heads: int = 4, dropout: float = 0.1):
        super().__init__()
        assert out_dim % num_heads == 0, "out_dim must be divisible by num_heads"
        self.num_heads = num_heads
        self.head_dim = out_dim // num_heads
        self.out_dim = out_dim

        self.W = nn.Linear(in_dim, out_dim, bias=False)
        self.a_src = nn.Parameter(torch.zeros(num_heads, self.head_dim))
        self.a_dst = nn.Parameter(torch.zeros(num_heads, self.head_dim))
        self.dropout = nn.Dropout(dropout)
        self.leaky_relu = nn.LeakyReLU(0.2)

        self._init_params()

    def _init_params(self):
        nn.init.xavier_uniform_(self.W.weight)
        nn.init.xavier_uniform_(self.a_src.unsqueeze(0))
        nn.init.xavier_uniform_(self.a_dst.unsqueeze(0))

    def forward(self, x: torch.Tensor, adj: torch.Tensor) -> torch.Tensor:
        """Forward pass.

        Args:
            x: Node features [batch, num_nodes, in_dim]
            adj: Adjacency matrix [batch, num_nodes, num_nodes] or [num_nodes, num_nodes]

        Returns:
            Updated node features [batch, num_nodes, out_dim]
        """
        B, N, _ = x.shape

        # Project: [B, N, out_dim] -> [B, N, heads, head_dim]
        h = self.W(x).view(B, N, self.num_heads, self.head_dim)

        # Attention scores
        # a_src, a_dst: [heads, head_dim]
        # score_src: [B, N, heads] = sum over head_dim
        score_src = (h * self.a_src).sum(dim=-1)  # [B, N, heads]
        score_dst = (h * self.a_dst).sum(dim=-1)  # [B, N, heads]

        # Pairwise attention: e_ij = LeakyReLU(score_src_i + score_dst_j)
        # [B, N, 1, heads] + [B, 1, N, heads] -> [B, N, N, heads]
        e = self.leaky_relu(score_src.unsqueeze(2) + score_dst.unsqueeze(1))

        # Mask non-adjacent nodes with -inf
        if adj.dim() == 2:
            mask = adj.unsqueeze(0).unsqueeze(-1)  # [1, N, N, 1]
        else:
            mask = adj.unsqueeze(-1)  # [B, N, N, 1]

        e = e.masked_fill(mask == 0, float("-inf"))

        # Softmax over neighbors
        alpha = F.softmax(e, dim=2)  # [B, N, N, heads]
        alpha = self.dropout(alpha)

        # Weighted aggregation: [B, N, N, heads] x [B, N, heads, head_dim] -> [B, N, heads, head_dim]
        # Expand h for matmul: [B, 1, N, heads, head_dim]
        h_expanded = h.unsqueeze(1).expand(B, N, N, self.num_heads, self.head_dim)
        out = (alpha.unsqueeze(-1) * h_expanded).sum(dim=2)  # [B, N, heads, head_dim]

        # Concatenate heads
        out = out.reshape(B, N, self.out_dim)  # [B, N, out_dim]
        return out


class GATBlock(nn.Module):
    """GAT layer with residual connection and layer normalization."""

    def __init__(self, dim: int, num_heads: int = 4, dropout: float = 0.1):
        super().__init__()
        self.gat = GATLayer(dim, dim, num_heads=num_heads, dropout=dropout)
        self.norm = nn.LayerNorm(dim)
        self.ffn = nn.Sequential(
            nn.Linear(dim, dim * 2),
            nn.GELU(),
            nn.Dropout(dropout),
            nn.Linear(dim * 2, dim),
            nn.Dropout(dropout),
        )
        self.norm2 = nn.LayerNorm(dim)

    def forward(self, x: torch.Tensor, adj: torch.Tensor) -> torch.Tensor:
        # GAT with residual
        h = self.gat(x, adj)
        x = self.norm(x + h)
        # FFN with residual
        x = self.norm2(x + self.ffn(x))
        return x


class DiplomacyPolicyNet(nn.Module):
    """GNN-based policy network for Diplomacy order prediction.

    Takes a board state tensor [B, 81, 47] and adjacency matrix [81, 81],
    encodes province embeddings via GAT layers, then decodes per-unit
    order predictions.

    The decoder uses cross-attention from each unit's province embedding
    to all province embeddings, then projects to the order vocabulary.
    """

    def __init__(
        self,
        num_areas: int = 81,
        num_features: int = 47,
        hidden_dim: int = 256,
        num_gat_layers: int = 3,
        num_heads: int = 4,
        order_vocab_size: int = 169,
        num_powers: int = 7,
        dropout: float = 0.1,
    ):
        super().__init__()
        self.num_areas = num_areas
        self.hidden_dim = hidden_dim
        self.order_vocab_size = order_vocab_size

        # Input projection
        self.input_proj = nn.Sequential(
            nn.Linear(num_features, hidden_dim),
            nn.GELU(),
            nn.LayerNorm(hidden_dim),
        )

        # Power embedding (concatenated to help model condition on active power)
        self.power_embed = nn.Embedding(num_powers, hidden_dim)

        # GAT encoder blocks
        self.gat_blocks = nn.ModuleList([
            GATBlock(hidden_dim, num_heads=num_heads, dropout=dropout)
            for _ in range(num_gat_layers)
        ])

        # Order decoder: cross-attention from unit nodes to all nodes
        self.query_proj = nn.Linear(hidden_dim, hidden_dim)
        self.key_proj = nn.Linear(hidden_dim, hidden_dim)
        self.value_proj = nn.Linear(hidden_dim, hidden_dim)
        self.attn_norm = nn.LayerNorm(hidden_dim)

        # Final order head: project to order vocabulary
        self.order_head = nn.Sequential(
            nn.Linear(hidden_dim, hidden_dim),
            nn.GELU(),
            nn.LayerNorm(hidden_dim),
            nn.Linear(hidden_dim, order_vocab_size),
        )

    def encode(self, board: torch.Tensor, adj: torch.Tensor) -> torch.Tensor:
        """Encode board state into province embeddings.

        Args:
            board: [B, 81, 47] board state tensor
            adj: [81, 81] adjacency matrix

        Returns:
            Province embeddings [B, 81, hidden_dim]
        """
        x = self.input_proj(board)  # [B, 81, hidden_dim]
        for block in self.gat_blocks:
            x = block(x, adj)
        return x

    def decode_orders(
        self,
        embeddings: torch.Tensor,
        unit_indices: torch.Tensor,
        power_indices: torch.Tensor,
    ) -> torch.Tensor:
        """Decode order logits for specified unit positions.

        Args:
            embeddings: Province embeddings [B, 81, hidden_dim]
            unit_indices: Indices of unit positions [B, max_units] (padded with -1)
            power_indices: Active power index [B]

        Returns:
            Order logits [B, max_units, order_vocab_size]
        """
        B, N, D = embeddings.shape
        max_units = unit_indices.shape[1]

        # Add power context to embeddings
        power_emb = self.power_embed(power_indices)  # [B, D]
        context = embeddings + power_emb.unsqueeze(1)  # [B, N, D]

        # Gather unit embeddings
        # Clamp negative indices to 0 for gathering (we'll mask later)
        safe_idx = unit_indices.clamp(min=0)  # [B, max_units]
        safe_idx_exp = safe_idx.unsqueeze(-1).expand(B, max_units, D)  # [B, max_units, D]
        unit_emb = torch.gather(context, 1, safe_idx_exp)  # [B, max_units, D]

        # Cross-attention: unit queries attend to all province keys
        Q = self.query_proj(unit_emb)  # [B, max_units, D]
        K = self.key_proj(context)     # [B, N, D]
        V = self.value_proj(context)   # [B, N, D]

        scale = math.sqrt(D)
        attn = torch.bmm(Q, K.transpose(1, 2)) / scale  # [B, max_units, N]
        attn = F.softmax(attn, dim=-1)
        attended = torch.bmm(attn, V)  # [B, max_units, D]

        # Residual + norm
        unit_repr = self.attn_norm(unit_emb + attended)

        # Project to order logits
        logits = self.order_head(unit_repr)  # [B, max_units, order_vocab_size]
        return logits

    def forward(
        self,
        board: torch.Tensor,
        adj: torch.Tensor,
        unit_indices: torch.Tensor,
        power_indices: torch.Tensor,
    ) -> torch.Tensor:
        """Full forward pass: encode board, decode orders.

        Args:
            board: [B, 81, 47]
            adj: [81, 81]
            unit_indices: [B, max_units] province indices of active units
            power_indices: [B] active power index

        Returns:
            Order logits [B, max_units, order_vocab_size]
        """
        embeddings = self.encode(board, adj)
        logits = self.decode_orders(embeddings, unit_indices, power_indices)
        return logits

    def count_parameters(self) -> int:
        """Return total number of trainable parameters."""
        return sum(p.numel() for p in self.parameters() if p.requires_grad)
