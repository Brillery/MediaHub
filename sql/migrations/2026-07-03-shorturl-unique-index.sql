-- MediaHub shorturl 非空原始 URL 唯一约束迁移草案。
--
-- 背景：
-- - shorturl 当前创建流程是先插入占位行，再把 short_key/original_url 回写到同一行。
-- - 因此不能直接给 original_url 建普通 UNIQUE KEY，否则多个并发占位行的空字符串会互相冲突。
-- - 本迁移使用 generated column 把空字符串转成 NULL，再对 generated column 建唯一索引。
-- - MySQL UNIQUE KEY 允许多行 NULL，这样既不破坏占位写入，又能约束真实 URL 不重复。
--
-- 上线要求：
-- - 先执行“重复数据检查”。
-- - 如果检查结果有行，必须人工确认保留哪条短链并清理重复记录后，再执行 ALTER。
-- - 本脚本不自动 DELETE 数据，避免误删线上短链。

USE mediahub;

-- 1. 检查公共短链是否存在同一 original_url 对应多条记录。
SELECT original_url, COUNT(*) AS duplicate_count
FROM url_map
WHERE original_url <> ''
GROUP BY original_url
HAVING COUNT(*) > 1;

-- 2. 检查用户短链是否存在同一 user_id + original_url 对应多条记录。
SELECT user_id, original_url, COUNT(*) AS duplicate_count
FROM url_map_user
WHERE original_url <> ''
GROUP BY user_id, original_url
HAVING COUNT(*) > 1;

-- 3. 如上面检查存在重复，用下面查询展开明细，由人工决定保留记录。
SELECT id, short_key, original_url, times, create_at, update_at
FROM url_map
WHERE original_url IN (
  SELECT original_url
  FROM (
    SELECT original_url
    FROM url_map
    WHERE original_url <> ''
    GROUP BY original_url
    HAVING COUNT(*) > 1
  ) AS duplicated_public_urls
)
ORDER BY original_url, id;

SELECT id, user_id, short_key, original_url, times, create_at, update_at
FROM url_map_user
WHERE (user_id, original_url) IN (
  SELECT user_id, original_url
  FROM (
    SELECT user_id, original_url
    FROM url_map_user
    WHERE original_url <> ''
    GROUP BY user_id, original_url
    HAVING COUNT(*) > 1
  ) AS duplicated_user_urls
)
ORDER BY user_id, original_url, id;

-- 4. 确认重复数据已经清理后，再执行以下 DDL。
-- original_url_unique 是非业务字段，只用于唯一约束：空字符串转 NULL，真实 URL 保持原值。
ALTER TABLE url_map
  ADD COLUMN original_url_unique VARCHAR(512)
    GENERATED ALWAYS AS (NULLIF(original_url, '')) STORED
    COMMENT '非空 original_url 唯一约束用生成列，空占位行转为 NULL',
  ADD UNIQUE INDEX uk_url_map_original_url_non_empty (original_url_unique);

ALTER TABLE url_map_user
  ADD COLUMN original_url_unique VARCHAR(512)
    GENERATED ALWAYS AS (NULLIF(original_url, '')) STORED
    COMMENT '非空 original_url 唯一约束用生成列，空占位行转为 NULL',
  ADD UNIQUE INDEX uk_url_map_user_original_url_non_empty (user_id, original_url_unique);
