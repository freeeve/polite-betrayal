"""Value network for Diplomacy position evaluation.

Shares the GAT encoder architecture from the policy network. Adds a value
head that predicts game outcomes from board positions for a given power.

Architecture:
  1. Shared GAT encoder: board [B, 81, 47] -> province embeddings [B, 81, 512]
  2. Attention-weighted global pooling over province embeddings
  3. Power-conditioned FC layers: 512 -> 256 -> 4 outputs
  4. Output per power: [normalized_sc_count, win_prob, draw_prob, survival_prob]
"""

import torch
import torch.nn as nn
import torch.nn.functional as F

from gnn import DiplomacyPolicyNet, GATBlock


class AttentionPooling(nn.Module):
    """Attention-weighted mean pooling over graph nodes.

    Learns a query vector that attends to all province embeddings,
    producing a single fixed-size graph-level representation.
    """

    def __init__(self, dim: int):
        super().__init__()
        self.attn = nn.Sequential(
            nn.Linear(dim, dim),
            nn.Tanh(),
            nn.Linear(dim, 1),
        )

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        """Pool node embeddings into a graph-level vector.

        Args:
            x: [B, N, D] node embeddings

        Returns:
            [B, D] graph-level embedding
        """
        scores = self.attn(x)  # [B, N, 1]
        weights = F.softmax(scores, dim=1)  # [B, N, 1]
        pooled = (x * weights).sum(dim=1)  # [B, D]
        return pooled


class DiplomacyValueNet(nn.Module):
    """Value network for Diplomacy position evaluation.

    Shares the GAT encoder with the policy network and adds a value head
    that predicts game outcomes (SC share, win/draw/survival probabilities)
    for a specified power.
    """

    def __init__(
        self,
        num_areas: int = 81,
        num_features: int = 47,
        hidden_dim: int = 512,
        num_gat_layers: int = 6,
        num_heads: int = 8,
        num_powers: int = 7,
        dropout: float = 0.1,
    ):
        super().__init__()
        self.hidden_dim = hidden_dim

        # Shared encoder components (same architecture as policy network)
        self.input_proj = nn.Sequential(
            nn.Linear(num_features, hidden_dim),
            nn.GELU(),
            nn.LayerNorm(hidden_dim),
        )
        self.gat_blocks = nn.ModuleList([
            GATBlock(hidden_dim, num_heads=num_heads, dropout=dropout)
            for _ in range(num_gat_layers)
        ])

        # Power embedding for conditioning
        self.power_embed = nn.Embedding(num_powers, hidden_dim)

        # Attention pooling: 81 province embeddings -> 1 graph embedding
        self.pool = AttentionPooling(hidden_dim)

        # Value head: graph embedding -> outcome predictions
        self.value_head = nn.Sequential(
            nn.Linear(hidden_dim, hidden_dim // 2),
            nn.GELU(),
            nn.Dropout(dropout),
            nn.Linear(hidden_dim // 2, 4),
        )

    def encode(self, board: torch.Tensor, adj: torch.Tensor) -> torch.Tensor:
        """Encode board state into province embeddings.

        Args:
            board: [B, 81, 47] board state tensor
            adj: [81, 81] adjacency matrix

        Returns:
            Province embeddings [B, 81, hidden_dim]
        """
        x = self.input_proj(board)
        for block in self.gat_blocks:
            x = block(x, adj)
        return x

    def forward(
        self,
        board: torch.Tensor,
        adj: torch.Tensor,
        power_indices: torch.Tensor,
    ) -> torch.Tensor:
        """Predict game outcomes for specified powers.

        Args:
            board: [B, 81, 47] board state tensor
            adj: [81, 81] adjacency matrix
            power_indices: [B] power index for each sample

        Returns:
            Value predictions [B, 4]:
              [0] predicted SC share (sigmoid, normalized to [0, 1])
              [1] win probability (sigmoid)
              [2] draw probability (sigmoid)
              [3] survival probability (sigmoid)
        """
        embeddings = self.encode(board, adj)

        # Add power context
        power_emb = self.power_embed(power_indices)  # [B, D]
        context = embeddings + power_emb.unsqueeze(1)  # [B, N, D]

        # Pool to graph-level representation
        graph_emb = self.pool(context)  # [B, D]

        # Predict value
        raw = self.value_head(graph_emb)  # [B, 4]
        return torch.sigmoid(raw)

    def load_encoder_from_policy(self, policy_net: DiplomacyPolicyNet):
        """Copy encoder weights from a trained policy network.

        Loads input_proj, gat_blocks, and power_embed weights from
        the policy network for transfer learning / shared training.
        """
        self.input_proj.load_state_dict(policy_net.input_proj.state_dict())
        self.gat_blocks.load_state_dict(policy_net.gat_blocks.state_dict())
        self.power_embed.load_state_dict(policy_net.power_embed.state_dict())

    def count_parameters(self) -> int:
        """Return total number of trainable parameters."""
        return sum(p.numel() for p in self.parameters() if p.requires_grad)
