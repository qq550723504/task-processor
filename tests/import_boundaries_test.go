	}
}

func TestBusinessImplementationPackagesDoNotImportGinDirectly(t *testing.T) {
	root := filepath.Join("..", "internal")
	allowedHTTPPackages := map[string]struct{}{
		filepath.Clean(filepath.Join(root, "app", "httpapi")) + string(os.PathSeparator):                              {},
		filepath.Clean(filepath.Join(root, "amazonlisting", "api")) + string(os.PathSeparator):                        {},
		filepath.Clean(filepath.Join(root, "amazonlisting", "httpapi")) + string(os.PathSeparator):                    {},
		filepath.Clean(filepath.Join(root, "httproute")) + string(os.PathSeparator):                                   {},
		filepath.Clean(filepath.Join(root, "kernel", "module")) + string(os.PathSeparator):                            {},
		filepath.Clean(filepath.Join(root, "listingkit", "api")) + string(os.PathSeparator):                           {},
		filepath.Clean(filepath.Join(root, "listingkit", "httpapi")) + string(os.PathSeparator):                       {},
		filepath.Clean(filepath.Join(root, "product", "sourcehandoff", "a1688", "httpapi")) + string(os.PathSeparator): {},
		filepath.Clean(filepath.Join(root, "productimage", "httpapi")) + string(os.PathSeparator):                     {},
		filepath.Clean(filepath.Join(root, "productenrich", "api")) + string(os.PathSeparator):                        {},
		filepath.Clean(filepath.Join(root, "productenrich", "httpapi")) + string(os.PathSeparator):                    {},
		filepath.Clean(filepath.Join(root, "promptmgmt", "api")) + string(os.PathSeparator):                           {},
		filepath.Clean(filepath.Join(root, "sds", "httpapi")) + string(os.PathSeparator):                              {},
		filepath.Clean(filepath.Join(root, "sdslogin")) + string(os.PathSeparator):                                    {},
		filepath.Clean(filepath.Join(root, "sheinlogin")) + string(os.PathSeparator):                                  {},
		filepath.Clean(filepath.Join(root, "taskrpcapi")) + string(os.PathSeparator):                                  {},
		filepath.Clean(filepath.Join(root, "amazonlisting", "interfaces.go")):                                         {},
		filepath.Clean(filepath.Join(root, "listingadmin", "category_handler.go")):                                    {},
		filepath.Clean(filepath.Join(root, "listingadmin", "filter_rule_handler.go")):                                 {},
		filepath.Clean(filepath.Join(root, "listingadmin", "generation_topic_catalog_handler.go")):                    {},
		filepath.Clean(filepath.Join(root, "listingadmin", "generation_topic_override_handler.go")):                   {},
		filepath.Clean(filepath.Join(root, "listingadmin", "generation_topic_policy_handler.go")):                     {},
		filepath.Clean(filepath.Join(root, "listingadmin", "handler_helpers.go")):                                     {},
		filepath.Clean(filepath.Join(root, "listingadmin", "import_task_handler.go")):                                 {},
		filepath.Clean(filepath.Join(root, "listingadmin", "operation_strategy_handler.go")):                          {},
		filepath.Clean(filepath.Join(root, "listingadmin", "pricing_rule_handler.go")):                                {},
		filepath.Clean(filepath.Join(root, "listingadmin", "product_data_handler.go")):                                {},
		filepath.Clean(filepath.Join(root, "listingadmin", "product_import_mapping.go")):                              {},
		filepath.Clean(filepath.Join(root, "listingadmin", "product_import_mapping_handler.go")):                      {},
		filepath.Clean(filepath.Join(root, "listingadmin", "profit_rule_handler.go")):                                 {},
		filepath.Clean(filepath.Join(root, "listingadmin", "request_context.go")):                                     {},
		filepath.Clean(filepath.Join(root, "listingadmin", "scheduled_task_config_handler.go")):                       {},
		filepath.Clean(filepath.Join(root, "listingadmin", "sensitive_word_handler.go")):                              {},
		filepath.Clean(filepath.Join(root, "listingadmin", "store_handler.go")):                                       {},
		filepath.Clean(filepath.Join(root, "listingadmin", "store_statistics_handler.go")):                            {},
		filepath.Clean(filepath.Join(root, "listingkit", "studio_session_handler.go")):                                {},
		filepath.Clean(filepath.Join(root, "listingsubscription", "handler.go")):                                      {},
		filepath.Clean(filepath.Join(root, "productenrich", "handler.go")):                                            {},
	}

	index, err := loadGoFileIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	for path, facts := range index.files {
		if strings.HasSuffix(filepath.Base(path), "_test.go") || pathAllowed(path, allowedHTTPPackages) {
			continue
		}
		if _, ok := facts.imports[`"github.com/gin-gonic/gin"`]; ok {
			t.Errorf("%s imports gin directly; keep HTTP framework dependencies in api/httpapi or explicitly registered legacy HTTP adapter packages", path)
		}
	}
}

func TestProductImageExternalClientImportsStayAllowlisted(t *testing.T) {
	root := filepath.Join("..", "internal", "productimage")
	allowedFiles := map[string]struct{}{