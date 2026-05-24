$env:TASK_PROCESSOR_DATABASE_HOST='127.0.0.1'
$env:TASK_PROCESSOR_DATABASE_PORT='15432'
$env:TASK_PROCESSOR_REDIS_HOST='127.0.0.1'
$env:TASK_PROCESSOR_REDIS_PORT='16379'
$env:TASK_PROCESSOR_SHEIN_COOKIE_REDIS_HOST='127.0.0.1'
$env:TASK_PROCESSOR_SHEIN_COOKIE_REDIS_PORT='16379'
Set-Location 'D:\code\task-processor'
go run ./cmd/product-listing-api -config config/config-dev.yaml -port 8085 -log-level info
