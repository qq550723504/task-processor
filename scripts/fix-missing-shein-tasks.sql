-- 临时修复：将缺少 shein 字段的 completed 任务改回 pending 状态
-- 这样 Temporal Worker 会自动重新处理这些任务

-- 第一步：找出所有需要修复的任务
-- 注意：这个查询会返回所有 completed 状态且 Result 中没有 shein 字段的任务
SELECT id, status 
FROM listing_kit_tasks 
WHERE status = 'completed' 
  AND result::jsonb ? 'pod_execution'
  AND (result::jsonb->'pod_execution')->>'status' = 'succeeded'
  AND NOT (result::jsonb ? 'shein');

-- 第二步：将这些任务的状态改回 pending（取消注释下面这行来执行）
-- UPDATE listing_kit_tasks 
-- SET status = 'pending', 
--     error = '', 
--     updated_at = NOW()
-- WHERE status = 'completed' 
--   AND result::jsonb ? 'pod_execution'
--   AND (result::jsonb->'pod_execution')->>'status' = 'succeeded'
--   AND NOT (result::jsonb ? 'shein');
