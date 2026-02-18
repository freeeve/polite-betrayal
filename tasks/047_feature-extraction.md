# Feature Extraction: Board Tensors and Order Labels

## Status: Pending

## Dependencies
- 046 (Data pipeline — needs parsed game data)

## Description
Build the feature extraction pipeline that converts parsed game data into training-ready tensors. This produces the input/output pairs for supervised learning.

1. **Board state tensor** (per section 4.6 of the plan):
   - 81 areas (75 provinces + 6 bicoastal variants: spa/nc, spa/sc, stp/nc, stp/sc, bul/ec, bul/sc)
   - 36 features per area:
     - Unit present: [army, fleet, empty] (3)
     - Unit owner: [A,E,F,G,I,R,T,none] (8)
     - SC owner: [A,E,F,G,I,R,T,neutral,none] (9)
     - Can build: bool (1)
     - Can disband: bool (1)
     - Dislodged: [army, fleet, none] (3)
     - Dislodged owner: [A,E,F,G,I,R,T,none] (8)
     - Province type: [land, sea, coast] (3)
   - Shape: [81, 36] = 2,916 floats per position

2. **Order labels**:
   - Per-unit order index into the legal order vocabulary (~200 possible orders per unit)
   - One-hot encoding over legal orders
   - Handle: which orders were actually played by human players

3. **Value labels**:
   - Final SC distribution at game end (7 values, sum to 34)
   - Win/draw/loss indicator per power
   - Normalized share (0.0 to 1.0)

4. **Previous orders feature** (optional context):
   - Encode last phase's orders as additional input
   - Same 81-area layout but with order-type indicators

5. **Dataset splits**:
   - Train/validation/test: 90/5/5 split
   - Balanced by power (each power equally represented)
   - Stratified by game year (early, mid, late game)

6. **Output format**: HDF5 or NumPy .npz files for efficient PyTorch DataLoader

## Acceptance Criteria
- Feature tensor shape is [81, 36] per position, correctly encoded
- At least 1M movement phase records extracted
- Order labels match actual played orders from game data
- Value labels derived from game outcomes
- Train/val/test split is reproducible (fixed random seed)
- Spot-check: manually verify 10 random examples against game records
- Loading a batch of 256 samples takes < 50ms

## Estimated Effort: M
