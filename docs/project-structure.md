# Task Processor 项目结构与函数清单

生成时间: 2026-03-10 14:38:36

## 目录结构

```文件夹 PATH 列表
卷序列号为 8AC1-3163
D:\CODE\TASK-PROCESSOR\INTERNAL
|   README.md
|   
+---app
|   |   README.md
|   |   
|   +---bootstrap
|   |       app.go
|   |       component_adapters.go
|   |       platform_processors.go
|   |       README.md
|   |       service_registry_simple.go
|   |       
|   +---messaging
|   |       crawler_registry.go
|   |       platform_registry.go
|   |       queue_config.go
|   |       queue_initializer.go
|   |       rabbitmq_publisher_adapter.go
|   |       rabbitmq_service.go
|   |       README.md
|   |       result_reporter.go
|   |       service_manager.go
|   |       task_handler.go
|   |       task_submitter.go
|   |       
|   +---processor
|   |       base_processor.go
|   |       base_task_handler.go
|   |       crawler_processor.go
|   |       interfaces.go
|   |       README.md
|   |       
|   +---scheduler
|   |       executor_stats.go
|   |       locked_task_executor.go
|   |       manager.go
|   |       manager_with_lock.go
|   |       monitor_service.go
|   |       README.md
|   |       registry.go
|   |       task_dependency.go
|   |       task_executor.go
|   |       types.go
|   |       
|   +---service
|   |       auth_service.go
|   |       processor_lifecycle.go
|   |       processor_manager.go
|   |       processor_service.go
|   |       processor_service_impl.go
|   |       README.md
|   |       scheduler_factory_creator.go
|   |       scheduler_platform_config.go
|   |       scheduler_service.go
|   |       scheduler_service_impl.go
|   |       scheduler_task_starter.go
|   |       status_monitor.go
|   |       
|   +---task
|   |       cleanup_service.go
|   |       deduplication_manager.go
|   |       dispatcher.go
|   |       fetcher.go
|   |       fetcher_utils.go
|   |       interfaces.go
|   |       models.go
|   |       monitor_service.go
|   |       processor_submitter_adapter.go
|   |       queue_manager.go
|   |       README.md
|   |       
|   +---updater
|   |       config.go
|   |       file_downloader.go
|   |       file_manager.go
|   |       models.go
|   |       updater.go
|   |       update_manager.go
|   |       utils.go
|   |       version_manager.go
|   |       
|   \---worker
|           job_handler.go
|           
+---application
|   +---crawler
|   |       crawler_types.go
|   |       distributed_crawler_client.go
|   |       result_listener.go
|   |       
|   +---product
|   |       distributed_fetcher.go
|   |       fetcher_factory.go
|   |       
|   \---state
|           cookie_manager.go
|           daily_count_manager.go
|           manager.go
|           README.md
|           relisting_queue_manager.go
|           shop_pause_manager.go
|           
+---core
|   |   README.md
|   |   
|   +---config
|   |   |   builder.go
|   |   |   common_types.go
|   |   |   common_types_test.go
|   |   |   config.go
|   |   |   config_test.go
|   |   |   defaults.go
|   |   |   defaults_applier.go
|   |   |   helpers.go
|   |   |   loader.go
|   |   |   loader_interface.go
|   |   |   manager.go
|   |   |   platform_common.go
|   |   |   platform_common_test.go
|   |   |   platform_registry.go
|   |   |   source.go
|   |   |   types.go
|   |   |   utils.go
|   |   |   validator.go
|   |   |   
|   |   +---loaders
|   |   |   |   builder.go
|   |   |   |   helpers.go
|   |   |   |   platform.go
|   |   |   |   rabbitmq.go
|   |   |   |   
|   |   |   \---middleware
|   |   |           README.md
|   |   |           
|   |   +---platforms
|   |   |       README.md
|   |   |       
|   |   +---sources
|   |   |       README.md
|   |   |       
|   |   +---types
|   |   |       amazon.go
|   |   |       browser.go
|   |   |       config.go
|   |   |       management.go
|   |   |       openai.go
|   |   |       platform.go
|   |   |       processor.go
|   |   |       rabbitmq.go
|   |   |       updater.go
|   |   |       worker.go
|   |   |       
|   |   +---utils
|   |   |       file.go
|   |   |       merge.go
|   |   |       path.go
|   |   |       
|   |   \---validators
|   |           amazon.go
|   |           browser.go
|   |           error.go
|   |           management.go
|   |           openai.go
|   |           platform.go
|   |           validator.go
|   |           worker.go
|   |           
|   +---errors
|   |       errors.go
|   |       examples_test.go
|   |       helpers.go
|   |       helpers_test.go
|   |       
|   +---lifecycle
|   |       component.go
|   |       interfaces.go
|   |       manager_impl.go
|   |       
|   +---logger
|   |   |   context.go
|   |   |   helpers.go
|   |   |   helpers_test.go
|   |   |   manager.go
|   |   |   manager_test.go
|   |   |   README.md
|   |   |   rotating_writer.go
|   |   |   
|   |   \---logs
|   |           app.log
|   |           
|   \---system
|           initializer.go
|           README.md
|           
+---crawler
|   |   README.md
|   |   
|   +---alibaba1688
|   |   |   browser_manager.go
|   |   |   captcha_handler.go
|   |   |   captcha_human_behavior.go
|   |   |   captcha_other.go
|   |   |   captcha_slider.go
|   |   |   captcha_types.go
|   |   |   page_operator.go
|   |   |   processor.go
|   |   |   product_checker.go
|   |   |   single_processor.go
|   |   |   url_helper.go
|   |   |   
|   |   +---extractor
|   |   |       attribute_extractor.go
|   |   |       base_extractor.go
|   |   |       basic_info_extractor.go
|   |   |       detail_extractor.go
|   |   |       image_extractor.go
|   |   |       pack_info_extractor.go
|   |   |       price_extractor.go
|   |   |       product_extractor.go
|   |   |       shipping_extractor.go
|   |   |       specification_extractor.go
|   |   |       supplier_extractor.go
|   |   |       title_extractor.go
|   |   |       variant_extractor.go
|   |   |       variant_values_extractor.go
|   |   |       
|   |   \---model
|   |           product.go
|   |           
|   +---amazon
|   |   |   batch_processor.go
|   |   |   captcha_handler.go
|   |   |   factory.go
|   |   |   instance_processor.go
|   |   |   processor.go
|   |   |   processor_wrapper.go
|   |   |   product_checker.go
|   |   |   single_processor.go
|   |   |   timeout_manager.go
|   |   |   url_helper.go
|   |   |   url_helper_test.go
|   |   |   
|   |   +---browser
|   |   |       browser_pool.go
|   |   |       config_manager.go
|   |   |       error_detector.go
|   |   |       health_checker.go
|   |   |       instance_manager.go
|   |   |       manager.go
|   |   |       pool_manager.go
|   |   |       selectors.go
|   |   |       zipcode_getter.go
|   |   |       zipcode_input_handler.go
|   |   |       zipcode_setter.go
|   |   |       zipcode_strategy.go
|   |   |       zipcode_strategy_city_dropdown.go
|   |   |       zipcode_strategy_japanese.go
|   |   |       zipcode_strategy_standard.go
|   |   |       zipcode_utils.go
|   |   |       zipcode_validator.go
|   |   |       
|   |   +---extractor
|   |   |       availability_extractor.go
|   |   |       availability_test.go
|   |   |       bestseller_extractor.go
|   |   |       brand_extractor.go
|   |   |       brand_extractor_test.go
|   |   |       categories_extractor.go
|   |   |       currency_manager.go
|   |   |       delivery_extractor.go
|   |   |       description_extractor.go
|   |   |       error_detector.go
|   |   |       extractor.go
|   |   |       features_extractor.go
|   |   |       feature_extractor.go
|   |   |       feature_parser_extractor.go
|   |   |       image_extractor.go
|   |   |       list_price_extractor.go
|   |   |       list_price_extractor_test.go
|   |   |       parent_asin_extractor.go
|   |   |       price_extractor.go
|   |   |       price_parser.go
|   |   |       price_validator.go
|   |   |       product_details_extractor.go
|   |   |       rating_extractor.go
|   |   |       seller_extractor.go
|   |   |       ships_from_extractor.go
|   |   |       text_cleaner.go
|   |   |       text_validator.go
|   |   |       title_extractor.go
|   |   |       variations_extractor.go
|   |   |       video_extractor.go
|   |   |       
|   |   \---variations
|   |           combinator.go
|   |           config.go
|   |           extractor.go
|   |           mapper.go
|   |           matcher.go
|   |           parser.go
|   |           types.go
|   |           
|   \---shared
|       \---browser
|               chrome_downloader.go
|               fingerprint.go
|               installer.go
|               launcher.go
|               launcher_context.go
|               manager.go
|               random_config.go
|               utils.go
|               
+---domain
|   |   README.md
|   |   
|   +---errors
|   |       task_errors.go
|   |       
|   +---message
|   |       types.go
|   |       
|   +---model
|   |       amazon_product.go
|   |       task.go
|   |       task_status.go
|   |       
|   +---product
|   |   |   cache_manager.go
|   |   |   crawler_client.go
|   |   |   data_parser.go
|   |   |   domain_resolver.go
|   |   |   fetcher.go
|   |   |   price_helper.go
|   |   |   
|   |   +---factory
|   |   |       product_factory.go
|   |   |       
|   |   +---repo
|   |   |   |   cache_repository.go
|   |   |   |   
|   |   |   \---impl
|   |   |           cache_repository_impl.go
|   |   |           crawler_repository_impl.go
|   |   |           
|   |   +---service
|   |   |       product_service.go
|   |   |       product_validator.go
|   |   |       
|   |   \---types
|   |           request.go
|   |           
|   +---queue
|   |       naming.go
|   |       
|   +---task
|   |       deduplicator.go
|   |       deduplicator_test.go
|   |       job.go
|   |       message_adapter.go
|   |       message_adapter_test.go
|   |       README.md
|   |       
|   \---validation
|           fulfillment_helper.go
|           rule_checker.go
|           
+---infra
|   |   README.md
|   |   
|   +---auth
|   |       cleanup.go
|   |       client_credentials.go
|   |       interfaces.go
|   |       manager.go
|   |       session.go
|   |       token_fetcher.go
|   |       token_store.go
|   |       
|   +---clients
|   |   \---openai
|   |           client.go
|   |           context_manager.go
|   |           pool.go
|   |           types.go
|   |           
|   +---di
|   |       cache.go
|   |       container_impl.go
|   |       interfaces.go
|   |       registry.go
|   |       
|   +---http
|   |   \---middleware
|   |           auth.go
|   |           logging.go
|   |           
|   +---lock
|   |       distributed_lock.go
|   |       memory_lock.go
|   |       memory_lock_test.go
|   |       
|   +---monitoring
|   |       collector.go
|   |       health_checker.go
|   |       metric_operations.go
|   |       process_info.go
|   |       README.md
|   |       types.go
|   |       
|   +---rabbitmq
|   |       client.go
|   |       client_test.go
|   |       config.go
|   |       config_test.go
|   |       connection.go
|   |       consumer.go
|   |       consumer_state.go
|   |       error_collector.go
|   |       load_monitor.go
|   |       load_monitor_test.go
|   |       message.go
|   |       message_test.go
|   |       queue_consumer.go
|   |       README.md
|   |       retry_strategy.go
|   |       retry_strategy_test.go
|   |       sliding_window_stats.go
|   |       sliding_window_stats_test.go
|   |       
|   +---repo
|   |       file_repo.go
|   |       
|   \---worker
|           config.go
|           interfaces.go
|           metrics.go
|           pool.go
|           README.md
|           types.go
|           worker.go
|           
+---pipeline
|   |   base_handler.go
|   |   context_impl.go
|   |   context_interfaces.go
|   |   errors.go
|   |   interfaces.go
|   |   parallel_handler.go
|   |   pipeline.go
|   |   README.md
|   |   
|   \---handlers
|           init_handler.go
|           logging_handler.go
|           validation_handler.go
|           
+---pkg
|   |   README.md
|   |   
|   +---amazon
|   |       domain_resolver.go
|   |       
|   +---downloader
|   |       anti_bot.go
|   |       image_downloader.go
|   |       image_processor.go
|   |       
|   +---management
|   |   |   category_restriction_cache.go
|   |   |   client_manager.go
|   |   |   management_api_client.go
|   |   |   sensitive_word_cache.go
|   |   |   
|   |   +---api
|   |   |       activity_product.go
|   |   |       activity_registration.go
|   |   |       category_restriction_collections_api.go
|   |   |       daily_listing_count_interface.go
|   |   |       filter_rule_interface.go
|   |   |       image_downloader.go
|   |   |       import_task.go
|   |   |       inventory_record.go
|   |   |       operation_strategy.go
|   |   |       pricing_rule_interface.go
|   |   |       product_data.go
|   |   |       product_import_mapping.go
|   |   |       profit_rule_interface.go
|   |   |       raw_json_data.go
|   |   |       sensitive_word_interface.go
|   |   |       store.go
|   |   |       
|   |   \---impl
|   |           activity_product_api.go
|   |           activity_registration_api.go
|   |           anti_bot_manager.go
|   |           base_management_api_client.go
|   |           block_detector.go
|   |           category_restriction_collections_api.go
|   |           daily_listing_count_api.go
|   |           filter_rule_api.go
|   |           http_client.go
|   |           image_downloader.go
|   |           image_download_processor.go
|   |           import_task_api.go
|   |           inventory_record_api.go
|   |           operation_strategy_client.go
|   |           pricing_rule_api.go
|   |           product_import_mapping_api.go
|   |           product_repository.go
|   |           profit_rule_api.go
|   |           rate_limit.go
|   |           raw_json_data_api.go
|   |           sensitive_word_api.go
|   |           store_api.go
|   |           
|   +---mathutil
|   |       math.go
|   |       math_test.go
|   |       
|   +---pricing
|   |       cost_calculator.go
|   |       cost_calculator_test.go
|   |       
|   +---ptrutil
|   |       pointer.go
|   |       pointer_test.go
|   |       
|   +---strutil
|   |       string.go
|   |       string_test.go
|   |       
|   +---types
|   |       flexible.go
|   |       
|   \---utils
|           cache.go
|           context.go
|           domain_utils.go
|           errors.go
|           file_utils.go
|           file_utils_test.go
|           goroutine_manager.go
|           goroutine_safe.go
|           goroutine_safe_test.go
|           hash_utils.go
|           help_utils.go
|           http_client.go
|           instance_utils.go
|           logger.go
|           logger_helper.go
|           metrics.go
|           parallel_processor.go
|           performance_tracker.go
|           platform_utils.go
|           shutdown.go
|           simple_logger.go
|           sku_generator.go
|           sku_generator_test.go
|           task_metrics.go
|           task_parser.go
|           text_cleaner.go
|           text_cleaner_test.go
|           url_utils.go
|           version_utils.go
|           worker_pool.go
|           
\---platforms
    |   README.md
    |   
    +---amazon
    |   |   processor.go
    |   |   
    |   +---api
    |   |       auth.go
    |   |       aws_signer.go
    |   |       catalog.go
    |   |       client.go
    |   |       inventory.go
    |   |       listings.go
    |   |       listing_details.go
    |   |       listing_models.go
    |   |       listing_operations.go
    |   |       listing_postman_test.go
    |   |       pricing.go
    |   |       product_types.go
    |   |       ratelimit.go
    |   |       request.go
    |   |       retry.go
    |   |       seller.go
    |   |       uploads.go
    |   |       
    |   +---internal
    |   |   +---handler
    |   |   |       attribute_mapper.go
    |   |   |       attribute_mapper_test.go
    |   |   |       base.go
    |   |   |       data_parser.go
    |   |   |       image.go
    |   |   |       interfaces.go
    |   |   |       inventory.go
    |   |   |       listing.go
    |   |   |       manager.go
    |   |   |       manager_test.go
    |   |   |       pricing.go
    |   |   |       product_data.go
    |   |   |       product_type.go
    |   |   |       store_info.go
    |   |   |       upload_flow_test.go
    |   |   |       validation.go
    |   |   |       variant.go
    |   |   |       
    |   |   +---model
    |   |   |       context.go
    |   |   |       errors.go
    |   |   |       product.go
    |   |   |       product_data.go
    |   |   |       product_types.go
    |   |   |       schema.go
    |   |   |       task_context.go
    |   |   |       variant.go
    |   |   |       
    |   |   \---service
    |   |           attribute_builder.go
    |   |           basic_attribute_builder.go
    |   |           custom_attribute_builder.go
    |   |           default_value_provider.go
    |   |           dynamic_template.go
    |   |           image_attribute_builder.go
    |   |           image_management.go
    |   |           image_processor.go
    |   |           llm_attribute_mapper.go
    |   |           llm_attribute_mapper_test.go
    |   |           openai_llm_client.go
    |   |           product_identifier_service.go
    |   |           product_type_recommendation.go
    |   |           required_attribute_builder.go
    |   |           s3_uploader.go
    |   |           schema_builder.go
    |   |           schema_fetcher.go
    |   |           schema_manager.go
    |   |           schema_parser.go
    |   |           service_factory.go
    |   |           variation_handler.go
    |   |           
    |   \---utils
    |           attribute_mapper.go
    |           attribute_validator.go
    |           converter.go
    |           identifier_generator.go
    |           validator.go
    |           variant_extractor.go
    |           
    +---common
    |   \---scheduler
    |           auto_pricing_task.go
    |           auto_pricing_task_test.go
    |           base_task.go
    |           base_task_test.go
    |           inventory_sync_task.go
    |           inventory_sync_task_test.go
    |           product_sync_task.go
    |           product_sync_task_test.go
    |           README.md
    |           
    +---shein
    |   +---api
    |   |   |   interface.go
    |   |   |   types.go
    |   |   |   
    |   |   +---attribute
    |   |   |       interface.go
    |   |   |       
    |   |   +---category
    |   |   |       interface.go
    |   |   |       
    |   |   +---image
    |   |   |       interface.go
    |   |   |       
    |   |   +---marketing
    |   |   |       activity_types.go
    |   |   |       interface.go
    |   |   |       price_types.go
    |   |   |       promotion_types.go
    |   |   |       README.md
    |   |   |       workflow_example.go
    |   |   |       
    |   |   +---other
    |   |   |       interface.go
    |   |   |       
    |   |   +---pricing
    |   |   |       interface.go
    |   |   |       
    |   |   +---product
    |   |   |       cost_price.go
    |   |   |       interface.go
    |   |   |       inventory.go
    |   |   |       model.go
    |   |   |       price.go
    |   |   |       request.go
    |   |   |       response.go
    |   |   |       shelf.go
    |   |   |       skc_sku.go
    |   |   |       stock.go
    |   |   |       
    |   |   +---translate
    |   |   |       interface.go
    |   |   |       
    |   |   \---warehouse
    |   |           interface.go
    |   |           
    |   +---model
    |   |       attribute.go
    |   |       errors.go
    |   |       inventory.go
    |   |       json_map.go
    |   |       module_errors.go
    |   |       module_models.go
    |   |       module_types.go
    |   |       product.go
    |   |       
    |   +---repo
    |   |   |   attribute_repo.go
    |   |   |   category_repo.go
    |   |   |   image_repo.go
    |   |   |   inventory_repo.go
    |   |   |   marketing_repo.go
    |   |   |   marketing_repo_interface.go
    |   |   |   other_repo.go
    |   |   |   price_manager.go
    |   |   |   pricing_repo.go
    |   |   |   product_manager.go
    |   |   |   product_repo.go
    |   |   |   product_repo_interface.go
    |   |   |   translate_repo.go
    |   |   |   warehouse_repo.go
    |   |   |   
    |   |   \---client
    |   |           api_client.go
    |   |           base_client.go
    |   |           cookie_manager.go
    |   |           endpoint.go
    |   |           endpoints.go
    |   |           error_handler.go
    |   |           manager.go
    |   |           
    |   +---scheduler
    |   |       activity_task.go
    |   |       auto_pricing_adapter.go
    |   |       auto_pricing_adapter_test.go
    |   |       base_task.go
    |   |       factory.go
    |   |       inventory_sync_adapter.go
    |   |       inventory_task.go
    |   |       pricing_task.go
    |   |       product_sync_adapter.go
    |   |       product_sync_adapter_test.go
    |   |       product_task.go
    |   |       
    |   +---service
    |   |   +---category
    |   |   |       ai_selector_service.go
    |   |   |       category.go
    |   |   |       manager_service.go
    |   |   |       restrictions_service.go
    |   |   |       tree_service.go
    |   |   |       
    |   |   +---common
    |   |   |   |   common.go
    |   |   |   |   result_merger.go
    |   |   |   |   
    |   |   |   +---data
    |   |   |   |       raw_json_service.go
    |   |   |   |       submit_service.go
    |   |   |   |       variant_data_service.go
    |   |   |   |       
    |   |   |   \---info
    |   |   |           site_service.go
    |   |   |           store_id_service.go
    |   |   |           store_info_service.go
    |   |   |           supplier_service.go
    |   |   |           warehouse_service.go
    |   |   |           
    |   |   +---content
    |   |   |       config_service.go
    |   |   |       optimizer_service.go
    |   |   |       processor_service.go
    |   |   |       text_cleaner.go
    |   |   |       utils_service.go
    |   |   |       validator_service.go
    |   |   |       words_processor_service.go
    |   |   |       word_service.go
    |   |   |       
    |   |   +---pipeline
    |   |   |       error_service.go
    |   |   |       handler_adapter.go
    |   |   |       pipeline_service.go
    |   |   |       processor_service.go
    |   |   |       router_service.go
    |   |   |       status_service.go
    |   |   |       submitter_service.go
    |   |   |       task_service.go
    |   |   |       
    |   |   +---pricing
    |   |   |       calculator.go
    |   |   |       cost_profit_calculator.go
    |   |   |       
    |   |   +---product
    |   |   |   |   init_data_service.go
    |   |   |   |   shelf_quota_service.go
    |   |   |   |   spu_limit_service.go
    |   |   |   |   spu_record_service.go
    |   |   |   |   
    |   |   |   +---attribute
    |   |   |   |   |   custom_processor.go
    |   |   |   |   |   fill_service.go
    |   |   |   |   |   importance_service.go
    |   |   |   |   |   mapper_service.go
    |   |   |   |   |   matcher_service.go
    |   |   |   |   |   prompt_service.go
    |   |   |   |   |   selector_service.go
    |   |   |   |   |   template_service.go
    |   |   |   |   |   utils_service.go
    |   |   |   |   |   validate_repair_service.go
    |   |   |   |   |   validator_service.go
    |   |   |   |   |   
    |   |   |   |   \---sale
    |   |   |   |           batch_processor_service.go
    |   |   |   |           comparison_service.go
    |   |   |   |           context_service.go
    |   |   |   |           debug_service.go
    |   |   |   |           file_saver_service.go
    |   |   |   |           filter_service.go
    |   |   |   |           gpt_service.go
    |   |   |   |           handler_service.go
    |   |   |   |           json_parser_service.go
    |   |   |   |           metadata_service.go
    |   |   |   |           preparation_service.go
    |   |   |   |           product_data_service.go
    |   |   |   |           prompt_generator_service.go
    |   |   |   |           request_service.go
    |   |   |   |           single_processor_service.go
    |   |   |   |           smart_filter_service.go
    |   |   |   |           utils_service.go
    |   |   |   |           validation_service.go
    |   |   |   |           value_filter_service.go
    |   |   |   |           
    |   |   |   +---build
    |   |   |   |       attribute_builder_service.go
    |   |   |   |       attribute_classifier_service.go
    |   |   |   |       build_attribute_service.go
    |   |   |   |       skc_list_service.go
    |   |   |   |       spu_service.go
    |   |   |   |       
    |   |   |   +---image
    |   |   |   |       processor_service.go
    |   |   |   |       validation_service.go
    |   |   |   |       
    |   |   |   +---skc
    |   |   |   |       attribute_strategy_service.go
    |   |   |   |       builder_service.go
    |   |   |   |       image_service.go
    |   |   |   |       translation_service.go
    |   |   |   |       validation_service.go
    |   |   |   |       variant_service.go
    |   |   |   |       
    |   |   |   +---sku
    |   |   |   |       builder_service.go
    |   |   |   |       creator_service.go
    |   |   |   |       generator_service.go
    |   |   |   |       image_fixer_service.go
    |   |   |   |       image_processor_service.go
    |   |   |   |       strategy_service.go
    |   |   |   |       utils_service.go
    |   |   |   |       
    |   |   |   \---variant
    |   |   |           composite_matcher.go
    |   |   |           exact_matcher.go
    |   |   |           fuzzy_matcher.go
    |   |   |           matcher.go
    |   |   |           utils.go
    |   |   |           
    |   |   +---publish
    |   |   |       checker_service.go
    |   |   |       error_handler_service.go
    |   |   |       exists_check_service.go
    |   |   |       handler_service.go
    |   |   |       result_service.go
    |   |   |       saver_service.go
    |   |   |       validator_service.go
    |   |   |       variant_success_service.go
    |   |   |       
    |   |   +---scheduler
    |   |   |       activity_config.go
    |   |   |       activity_errors.go
    |   |   |       activity_registration.go
    |   |   |       activity_registration_config.go
    |   |   |       activity_registration_mixed.go
    |   |   |       activity_registration_profit.go
    |   |   |       activity_stock_validation_test.go
    |   |   |       activity_time_limited_discount.go
    |   |   |       auto_pricing.go
    |   |   |       inventory_sync.go
    |   |   |       inventory_sync_amazon_fetcher.go
    |   |   |       inventory_sync_api.go
    |   |   |       inventory_sync_change_checker.go
    |   |   |       inventory_sync_config_getter.go
    |   |   |       inventory_sync_cost_calculator.go
    |   |   |       inventory_sync_helper.go
    |   |   |       inventory_sync_monitor.go
    |   |   |       inventory_sync_price_strategy.go
    |   |   |       inventory_sync_record.go
    |   |   |       inventory_sync_strategy.go
    |   |   |       inventory_sync_types.go
    |   |   |       mapping_repair_builder.go
    |   |   |       mapping_repair_handler.go
    |   |   |       mapping_repair_service.go
    |   |   |       mapping_repair_strategies.go
    |   |   |       mapping_repair_types.go
    |   |   |       price_calculator.go
    |   |   |       pricing_builder.go
    |   |   |       pricing_calculator.go
    |   |   |       pricing_evaluator.go
    |   |   |       product_data_helper.go
    |   |   |       product_sync.go
    |   |   |       product_sync_enricher.go
    |   |   |       product_sync_fetcher.go
    |   |   |       product_sync_types.go
    |   |   |       validation_utils.go
    |   |   |       validation_utils_test.go
    |   |   |       
    |   |   +---translate
    |   |   |       language_detector.go
    |   |   |       translate_service.go
    |   |   |       
    |   |   \---validation
    |   |           build_attribute_service.go
    |   |           daily_limit_service.go
    |   |           filter_rule_service.go
    |   |           get_rule_service.go
    |   |           quantity_service.go
    |   |           reapply_filter_service.go
    |   |           rule_checker_service.go
    |   |           
    |   \---utils
    |           data_enricher.go
    |           mapper.go
    |           monitor_helper.go
    |           sensitive_words_filter.go
    |           skip_helper.go
    |           string_sanitizer.go
    |           time_helper.go
    |           
    \---temu
        |   executor.go
        |   monitor_helper.go
        |   pipeline_builder.go
        |   pipeline_registry.go
        |   processor.go
        |   README.md
        |   task_handler.go
        |   task_submitter.go
        |   
        +---api
        |   |   api.go
        |   |   
        |   +---client
        |   |       auth.go
        |   |       auth_config.go
        |   |       auth_error_detector.go
        |   |       auth_factory.go
        |   |       auth_pause_handler.go
        |   |       auth_request_sender.go
        |   |       auth_retry_handler.go
        |   |       client.go
        |   |       config.go
        |   |       cookie_manager.go
        |   |       http.go
        |   |       interfaces.go
        |   |       manager.go
        |   |       
        |   +---models
        |   |       bulk_relist.go
        |   |       category.go
        |   |       certification.go
        |   |       common.go
        |   |       extension.go
        |   |       goods.go
        |   |       image.go
        |   |       inventory.go
        |   |       listing.go
        |   |       offline.go
        |   |       pricing.go
        |   |       product.go
        |   |       query.go
        |   |       sku.go
        |   |       sku_query.go
        |   |       submit.go
        |   |       
        |   \---services
        |           category.go
        |           image_upload.go
        |           inventory.go
        |           listing.go
        |           offline.go
        |           pricing.go
        |           product.go
        |           query.go
        |           sku_query.go
        |           submit.go
        |           
        +---context
        |       temu_context.go
        |       
        +---handlers
        |   |   README.md
        |   |   REFACTORING_STATUS.md
        |   |   
        |   +---ai
        |   |       content_rewriter.go
        |   |       prompt_builder.go
        |   |       property_mapper_core.go
        |   |       property_mapper_helpers.go
        |   |       property_validator.go
        |   |       service.go
        |   |       
        |   +---category
        |   |       disclaim_handler.go
        |   |       handler.go
        |   |       recommend_handler.go
        |   |       
        |   +---common
        |   |       base_handler.go
        |   |       filter_types.go
        |   |       fulfillment_checker.go
        |   |       init_handler.go
        |   |       models.go
        |   |       property_types.go
        |   |       sku_builder_interface.go
        |   |       temu_handler.go
        |   |       types.go
        |   |       
        |   +---filter
        |   |       prohibited_items_config.go
        |   |       prohibited_items_detector.go
        |   |       prohibited_items_detectors.go
        |   |       prohibited_items_types.go
        |   |       prohibited_items_utils.go
        |   |       prohibited_items_weapons.go
        |   |       rule_handler.go
        |   |       rule_manager.go
        |   |       rule_stats_provider.go
        |   |       sensitive_words_filter.go
        |   |       
        |   +---image
        |   |       carousel_builder.go
        |   |       dimension_annotator.go
        |   |       dimension_builder.go
        |   |       dimension_drawer.go
        |   |       dimension_models.go
        |   |       drawing_utils.go
        |   |       init_handler.go
        |   |       main_validator.go
        |   |       padding_processor.go
        |   |       parallel_processor.go
        |   |       parallel_validator.go
        |   |       processor.go
        |   |       single_validator.go
        |   |       sku_validator.go
        |   |       upload_helpers.go
        |   |       upload_processor.go
        |   |       upload_utils.go
        |   |       upload_worker.go
        |   |       validator.go
        |   |       vision_detector.go
        |   |       
        |   +---product
        |   |       brand_clear_handler.go
        |   |       build_spu_handler.go
        |   |       cache_handler.go
        |   |       check_daily_limit_handler.go
        |   |       commit_create_handler.go
        |   |       commit_detail_handler.go
        |   |       description_validator.go
        |   |       description_validator_enhance.go
        |   |       description_validator_rules.go
        |   |       description_validator_score.go
        |   |       exists_check_handler.go
        |   |       filter_checker.go
        |   |       name_optimizer.go
        |   |       name_validator.go
        |   |       out_goods_sn_check_handler.go
        |   |       price_handler.go
        |   |       price_query_handler.go
        |   |       raw_json_data_handler.go
        |   |       save_handler.go
        |   |       save_publish_result_handler.go
        |   |       spu_builder.go
        |   |       spu_validator.go
        |   |       submit_error_analyzer.go
        |   |       submit_fixer.go
        |   |       submit_handler.go
        |   |       submit_utils.go
        |   |       submit_validator.go
        |   |       
        |   +---property
        |   |       attribute_extractor.go
        |   |       cache.go
        |   |       conditional_validator.go
        |   |       context.go
        |   |       deduplicator.go
        |   |       default_filler.go
        |   |       feature_detector.go
        |   |       finalization_stage.go
        |   |       fixing_stage.go
        |   |       mapper_adapter.go
        |   |       mapping_stage.go
        |   |       mixed_attributes_processor.go
        |   |       orchestrator.go
        |   |       pipeline.go
        |   |       required_guardian.go
        |   |       selection_validator.go
        |   |       stage.go
        |   |       validation_config.go
        |   |       validation_stage.go
        |   |       validation_stats.go
        |   |       validator.go
        |   |       validator_condition.go
        |   |       value_fixer.go
        |   |       value_validator.go
        |   |       
        |   +---sku
        |   |       ai_mapping_batch_processor.go
        |   |       ai_mapping_handler.go
        |   |       ai_mapping_prompt_builder.go
        |   |       ai_mapping_response_processor.go
        |   |       ai_mapping_single_processor.go
        |   |       ai_mapping_utils_helper.go
        |   |       ai_mapping_validator.go
        |   |       builder.go
        |   |       cache_variants_handler.go
        |   |       item_builder.go
        |   |       mapping_processor.go
        |   |       parallel_builder.go
        |   |       parallel_variant_handler.go
        |   |       price_calculator.go
        |   |       skc_builder.go
        |   |       spec_handler.go
        |   |       utils.go
        |   |       variant_data_processor.go
        |   |       variant_filter_handler.go
        |   |       variant_json_data_handler.go
        |   |       variant_processor.go
        |   |       
        |   +---spec
        |   |       dimension_selector.go
        |   |       dimension_unifier.go
        |   |       resolver_service.go
        |   |       
        |   +---store
        |   |       id_handler.go
        |   |       info_handler.go
        |   |       
        |   +---template
        |   |       cost_handler.go
        |   |       query_handler.go
        |   |       
        |   \---validation
        |           bullet_points_optimizer.go
        |           bullet_points_validator.go
        |           percentage_sum_rule.go
        |           rule_engine.go
        |           rule_interface.go
        |           rule_validator.go
        |           rule_validator_test.go
        |           text_check_handler.go
        |           text_processor.go
        |           text_renderer.go
        |           
        +---scheduler
        |       activity_task.go
        |       auto_pricing_adapter.go
        |       auto_pricing_adapter_test.go
        |       base_task.go
        |       factory.go
        |       inventory_sync_adapter.go
        |       inventory_task.go
        |       pricing_task.go
        |       product_sync_adapter.go
        |       product_sync_adapter_test.go
        |       product_task.go
        |       
        +---service
        |   \---scheduler
        |           factory.go
        |           inventory_sync_amazon_fetcher.go
        |           inventory_sync_api.go
        |           inventory_sync_change_checker.go
        |           inventory_sync_concurrent.go
        |           inventory_sync_config_getter.go
        |           inventory_sync_cost_calculator.go
        |           inventory_sync_factory.go
        |           inventory_sync_helper.go
        |           inventory_sync_interface.go
        |           inventory_sync_monitor.go
        |           inventory_sync_record.go
        |           inventory_sync_service.go
        |           inventory_sync_strategy.go
        |           inventory_sync_updater.go
        |           mapping.go
        |           product_converter.go
        |           product_data_builder.go
        |           product_sync_service.go
        |           product_sync_types.go
        |           product_utils.go
        |           sku_details_handler.go
        |           sku_mapping_enricher.go
        |           
        +---services
        |   |   image_config_service.go
        |   |   image_upload_service.go
        |   |   image_validation_service.go
        |   |   
        |   +---pricing
        |   |       auto_pricing_service.go
        |   |       interfaces.go
        |   |       pricing_data_service.go
        |   |       pricing_decision_service.go
        |   |       pricing_rule_calculator.go
        |   |       store_config_service.go
        |   |       
        |   \---product
        |           bulk_relist_batch.go
        |           bulk_relist_entry.go
        |           bulk_relist_filter.go
        |           bulk_relist_page_loop.go
        |           bulk_relist_processor.go
        |           bulk_relist_service.go
        |           
        +---types
        |       ai_types.go
        |       errors.go
        |       property_mapping_types.go
        |       template_types.go
        |       
        \---utils
                format_example.go
                format_validator.go
                format_validator_test.go
                image_encoder.go
                image_utils.go
                image_validator.go
                ```

## Go 代码文件列表

总计: 932 个 Go 文件


