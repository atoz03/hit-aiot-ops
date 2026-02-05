-- 0002_seed_prices.sql：默认 GPU 单价（可根据集群硬件调整）

INSERT INTO resource_prices (gpu_model, price_per_minute)
VALUES
  ('CPU_CORE', 0.02),
  ('A100', 0.50),
  ('RTX 4090', 0.30),
  ('RTX 3090', 0.20),
  ('RTX 3080', 0.15),
  ('V100', 0.40)
ON CONFLICT (gpu_model) DO UPDATE
SET price_per_minute = EXCLUDED.price_per_minute,
    updated_at = NOW();
