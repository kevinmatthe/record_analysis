package main

import (
	"fmt"
	"os"
	"strings"

	appconfig "github.com/kevinmatthe/record_analysis/internal/config"
	"github.com/kevinmatthe/record_analysis/internal/server"
	"gorm.io/driver/postgres"
	"gorm.io/gen"
	"gorm.io/gorm"
)

func main() {
	path := appconfig.LoadPath()
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	cfg, err := appconfig.LoadFile(path)
	if err != nil {
		panic(err)
	}
	dsn := cfg.DBDSN()
	if dsn == "" {
		panic("db_config is required")
	}
	if _, err := server.NewPostgresStore(dsn); err != nil {
		panic(err)
	}
	g := gen.NewGenerator(gen.Config{
		OutPath: "internal/infrastructure/db/query",
		Mode:    gen.WithDefaultQuery | gen.WithQueryInterface | gen.WithGeneric,
	})
	gormdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	g.UseDB(gormdb)
	tableNames, err := gormdb.Migrator().GetTables()
	if err != nil {
		panic(err)
	}
	tables := make([]any, 0)
	for _, tableName := range tableNames {
		if strings.HasPrefix(tableName, "record_analysis_") {
			fmt.Println("generate", tableName)
			tables = append(tables, g.GenerateModel(tableName))
		}
	}
	g.ApplyBasic(tables...)
	g.Execute()
}
