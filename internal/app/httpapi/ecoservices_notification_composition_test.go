package httpapi

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/authz"
	b "task-processor/internal/commercial/billing"
	"task-processor/internal/core/config"
	e "task-processor/internal/ecoservices"
	kernelmodule "task-processor/internal/kernel/module"
)

type combinationObjects struct{ e.PrivateObjectStore }
type combinationProvider struct{ b.ServicePurchaseProvider }
type combinationProtection struct{ b.ServicePayloadProtection }

func TestCurrentApplicationEcoservicesAndNotificationOwnersCannotAlias(t *testing.T) {
	for _, alias := range []bool{false, true} {
		t.Run(map[bool]string{false: "independent", true: "aliased"}[alias], func(t *testing.T) {
			source, commercial, money, eco, notice := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			if alias {
				notice = eco
			}
			stop := errors.New("bounded workbench construction stop")
			built := 0
			factories := currentApplicationFactories{
				buildWorkbench: func(*config.Config, *logrus.Logger) (workbenchContextBuildResult, error) {
					built++
					return workbenchContextBuildResult{}, stop
				},
				buildSourceAccount: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
				buildCommercial:    func(*gorm.DB, *gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
			}
			_, err := buildCurrentApplication(context.Background(), source, currentApplicationTestConfig(), logrus.New(), factories,
				WithCommercialOwnerDatabase(commercial), WithMoneyOwnerDatabase(money),
				WithEcoservices(EcoservicesDependencies{DB: eco, Objects: &combinationObjects{}, Channel: &combinationProvider{}, Protection: &combinationProtection{}}),
				WithNotificationCenter(notice))
			if alias {
				if err == nil || !strings.Contains(err.Error(), "ecoservices requires its independent owner pool") || built != 0 {
					t.Fatalf("alias escaped option admission: err=%v built=%d", err, built)
				}
			} else if !errors.Is(err, stop) || built != 1 {
				t.Fatalf("independent options denied: %v", err)
			}
		})
	}
}

func TestNotificationModuleRejectsEcoservicesPoolBeforeSchemaRead(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	_, err = buildNotificationModule(context.Background(), db, currentApplicationTestConfig(), nil, nil,
		currentApplicationOptions{notifications: 1, notificationDB: db, ecoservices: &EcoservicesDependencies{DB: db}})
	if err == nil || !strings.Contains(err.Error(), "notification center cannot share an owner pool") {
		t.Fatalf("alias escaped pre-schema admission: %v", err)
	}
}
