package storage

import (
	"errors"
	"reflect"
	"strings"

	"go.uber.org/zap"
	"ysp.com/ncp/ncp/interfaces"
	"ysp.com/ncp/ncp/interfaces/dpcore/model"
	"ysp.com/ncp/ncp/interfaces/ncp"
	dpcorestorage "ysp.com/ncp/ncp/plugins/dpcore/storage"
)

var ErrSQLiteTableNotFound = errors.New("sqlite table not found")

type SqliteTableProvider struct {
	ctx ncp.Context
	log *zap.Logger
}

type managedTableQuery struct {
	sample  any
	typeTag string
	countFn func() (int64, error)
}
type SQLiteTableSummary struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	RowCount     int64    `json:"rowCount"`
	ColumnCount  int      `json:"columnCount"`
	PrimaryKeys  []string `json:"primaryKeys"`
	PrimaryCount int      `json:"primaryCount"`
}

type SQLiteTableDetail struct {
	Name        string           `json:"name"`
	Type        string           `json:"type"`
	Columns     []string         `json:"columns"`
	PrimaryKeys []string         `json:"primaryKeys"`
	Rows        []map[string]any `json:"rows"`
	RowCount    int64            `json:"rowCount"`
	Page        int              `json:"page"`
	PerPage     int              `json:"perPage"`
}

type SQLiteTableProvider interface {
	ListSQLiteTables() ([]SQLiteTableSummary, error)
	GetSQLiteTable(name string, page int, perPage int) (SQLiteTableDetail, error)
}

type sqliteTableReader interface {
	ReadSQLiteTable(tableName string, page int, perPage int) ([]string, []map[string]any, int64, error)
}

func NewSQLiteTableProvider(ctx ncp.Context, log *zap.Logger) *SqliteTableProvider {
	if log == nil {
		log = zap.NewNop()
	}

	return &SqliteTableProvider{
		ctx: ctx,
		log: log,
	}
}

func (p *SqliteTableProvider) ListSQLiteTables() ([]SQLiteTableSummary, error) {
	queries, err := p.managedTableQueries()
	if err != nil {
		return nil, err
	}

	items := make([]SQLiteTableSummary, 0, len(queries))
	for _, query := range queries {
		summary, err := buildManagedTableSummary(query)
		if err != nil {
			return nil, err
		}
		items = append(items, summary)
	}

	return items, nil
}

func (p *SqliteTableProvider) GetSQLiteTable(name string, page int, perPage int) (SQLiteTableDetail, error) {
	queries, err := p.managedTableQueries()
	if err != nil {
		return SQLiteTableDetail{}, err
	}

	query, ok := findManagedTableQuery(queries, name)
	if !ok {
		return SQLiteTableDetail{}, ErrSQLiteTableNotFound
	}

	tableName, expectedColumns, primaryKeys, err := inspectModelSchema(query.sample)
	if err != nil {
		return SQLiteTableDetail{}, err
	}

	reader, ok := p.ctx.GetInterface(interfaces.DpStorageName).(sqliteTableReader)
	if !ok {
		return SQLiteTableDetail{}, errors.New("dp storage is unavailable")
	}

	columns, rows, rowCount, err := reader.ReadSQLiteTable(tableName, page, perPage)
	if err != nil {
		if errors.Is(err, dpcorestorage.ErrSQLiteTableNotFound) {
			columns = expectedColumns
			rows = []map[string]any{}
			rowCount = 0
		} else {
			return SQLiteTableDetail{}, err
		}
	}

	if len(columns) == 0 {
		columns = expectedColumns
	}

	return SQLiteTableDetail{
		Name:        tableName,
		Type:        query.typeTag,
		Columns:     columns,
		PrimaryKeys: primaryKeys,
		Rows:        rows,
		RowCount:    rowCount,
		Page:        normalizePage(page),
		PerPage:     normalizePerPage(perPage),
	}, nil
}

func (p *SqliteTableProvider) managedTableQueries() ([]managedTableQuery, error) {
	idcInfoMng, ok := p.ctx.GetInterface(interfaces.DpIdcInfoStorageName).(model.IIdcInfoMng)
	if !ok {
		return nil, errors.New("dp idc info storage is unavailable")
	}

	productInfoMng, ok := p.ctx.GetInterface(interfaces.DPProductInfoName).(model.IProductRepository)
	if !ok {
		return nil, errors.New("dp product info storage is unavailable")
	}

	deployInfoMng, ok := p.ctx.GetInterface(interfaces.DpDeployInfoStorageName).(model.DeployRepository)
	if !ok {
		return nil, errors.New("dp deploy info storage is unavailable")
	}

	certInfoMng, ok := p.ctx.GetInterface(interfaces.DpCertInfoStorageName).(model.ICerInfoMng)
	if !ok {
		return nil, errors.New("dp cert info storage is unavailable")
	}

	topoInfoMng, ok := p.ctx.GetInterface(interfaces.DpTopoInfoStorageName).(model.ITopoInfoMng)
	if !ok {
		return nil, errors.New("dp topo info storage is unavailable")
	}

	operateLogMng, ok := p.ctx.GetInterface(interfaces.DpOperateLogStorageName).(model.IOperateLogMng)
	if !ok {
		return nil, errors.New("dp operate log storage is unavailable")
	}

	serviceGroupMng, ok := p.ctx.GetInterface(interfaces.DpServiceGroupMngStorageName).(model.IServiceGroupMng)
	if !ok {
		return nil, errors.New("dp service group storage is unavailable")
	}

	configRepo, ok := p.ctx.GetInterface(interfaces.DpConfigDataStorageName).(model.ConfigRepository)
	if !ok {
		return nil, errors.New("dp config data storage is unavailable")
	}

	var optRepo model.OptInfoRepository
	if repo, ok := p.ctx.GetInterface(interfaces.DpOperateLogStorageName).(model.OptInfoRepository); ok {
		optRepo = repo
	}

	queries := []managedTableQuery{
		{
			sample:  model.IdcInfo{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := idcInfoMng.GetIdcInfo()
				return int64(len(items)), err
			},
		},
		{
			sample:  model.ProductInfo{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := productInfoMng.GetAllProductInfo()
				return int64(len(items)), err
			},
		},
		{
			sample:  model.ProductVersionInfo{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := productInfoMng.GetAllProductVersion()
				return int64(len(items)), err
			},
		},
		{
			sample:  model.ProductPkgInfo{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := productInfoMng.GetAllProductPkgInfo()
				return int64(len(items)), err
			},
		},
		{
			sample:  model.DeployIpInfo{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := deployInfoMng.GetAllDeployIpInfo()
				return int64(len(items)), err
			},
		},
		{
			sample:  model.DeployPort{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := deployInfoMng.GetAllDeployPort()
				return int64(len(items)), err
			},
		},
		{
			sample:  model.DeployMicroPort{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := deployInfoMng.GetAllDeployMicroPort()
				return int64(len(items)), err
			},
		},
		{
			sample:  model.DeployFileInfo{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := deployInfoMng.GetAllDeployFileInfo()
				return int64(len(items)), err
			},
		},
		{
			sample:  model.CertInfo{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := certInfoMng.GetAllCertInfo()
				return int64(len(items)), err
			},
		},
		{
			sample:  model.TopoInfo{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := topoInfoMng.GetAllTopoInfo()
				return int64(len(items)), err
			},
		},
		{
			sample:  model.OperateLog{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				_, total, err := operateLogMng.FindAll(model.NewPageable().SetPage(0).SetSize(1))
				return total, err
			},
		},
		{
			sample:  model.ServiceGroupMng{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := serviceGroupMng.GetAllServiceGroupMng()
				return int64(len(items)), err
			},
		},
		{
			sample:  model.ServiceGroupInfo{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := serviceGroupMng.GetAllServiceGroupInfo()
				return int64(len(items)), err
			},
		},
		{
			sample:  model.ConfigDataInfo{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := configRepo.GetAllConfigDataInfo()
				return int64(len(items)), err
			},
		},
		{
			sample:  model.ConfigRegInfo{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := configRepo.GetAllConfigRegInfo()
				return int64(len(items)), err
			},
		},
	}

	if optRepo != nil {
		queries = append(queries, managedTableQuery{
			sample:  model.SystemParam{},
			typeTag: "managed",
			countFn: func() (int64, error) {
				items, err := optRepo.GetAllSystemParam()
				return int64(len(items)), err
			},
		})
	}

	return queries, nil
}

func buildManagedTableSummary(query managedTableQuery) (SQLiteTableSummary, error) {
	tableName, columnCount, primaryKeys, err := inspectModelDefinition(query.sample)
	if err != nil {
		return SQLiteTableSummary{}, err
	}

	rowCount, err := query.countFn()
	if err != nil {
		if isMissingSQLiteTableError(err) {
			rowCount = 0
		} else {
			return SQLiteTableSummary{}, err
		}
	}

	return SQLiteTableSummary{
		Name:         tableName,
		Type:         query.typeTag,
		RowCount:     rowCount,
		ColumnCount:  columnCount,
		PrimaryKeys:  primaryKeys,
		PrimaryCount: len(primaryKeys),
	}, nil
}

func findManagedTableQuery(queries []managedTableQuery, tableName string) (managedTableQuery, bool) {
	for _, query := range queries {
		name, _, _, err := inspectModelSchema(query.sample)
		if err != nil {
			continue
		}
		if name == tableName {
			return query, true
		}
	}

	return managedTableQuery{}, false
}

func inspectModelDefinition(sample any) (string, int, []string, error) {
	tableName, columns, primaryKeys, err := inspectModelSchema(sample)
	if err != nil {
		return "", 0, nil, err
	}

	return tableName, len(columns), primaryKeys, nil
}

func inspectModelSchema(sample any) (string, []string, []string, error) {
	type tableNamer interface {
		TableName() string
	}

	namer, ok := sample.(tableNamer)
	if !ok {
		return "", nil, nil, errors.New("model does not expose TableName")
	}

	fields := collectModelFields(reflect.TypeOf(sample))
	columns := make([]string, 0, len(fields))
	primaryKeys := make([]string, 0)
	for _, field := range fields {
		columns = append(columns, field.name)
		if field.primaryKey {
			primaryKeys = append(primaryKeys, field.name)
		}
	}

	return namer.TableName(), columns, primaryKeys, nil
}

type modelField struct {
	name       string
	primaryKey bool
}

func collectModelFields(modelType reflect.Type) []modelField {
	for modelType.Kind() == reflect.Pointer {
		modelType = modelType.Elem()
	}

	if modelType.Kind() != reflect.Struct {
		return nil
	}

	fields := make([]modelField, 0, modelType.NumField())
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)

		if field.PkgPath != "" {
			continue
		}

		gormTag := field.Tag.Get("gorm")
		if gormTag == "-" || strings.Contains(gormTag, "-;") || strings.Contains(gormTag, ";-") {
			continue
		}

		if field.Anonymous && shouldExpandEmbeddedField(field) {
			fields = append(fields, collectModelFields(field.Type)...)
			continue
		}

		fieldName := resolveColumnName(field)
		if fieldName == "" {
			continue
		}

		fields = append(fields, modelField{
			name:       fieldName,
			primaryKey: hasPrimaryKeyTag(field),
		})
	}

	return fields
}

func shouldExpandEmbeddedField(field reflect.StructField) bool {
	fieldType := field.Type
	for fieldType.Kind() == reflect.Pointer {
		fieldType = fieldType.Elem()
	}

	return fieldType.Kind() == reflect.Struct
}

func resolveColumnName(field reflect.StructField) string {
	gormTag := field.Tag.Get("gorm")
	for _, part := range strings.Split(gormTag, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "column:") {
			return strings.TrimPrefix(part, "column:")
		}
	}

	jsonTag := field.Tag.Get("json")
	if jsonTag != "" {
		name := strings.Split(jsonTag, ",")[0]
		if name != "" && name != "-" {
			return name
		}
	}

	return strings.ToLower(field.Name)
}

func hasPrimaryKeyTag(field reflect.StructField) bool {
	gormTag := field.Tag.Get("gorm")
	for _, part := range strings.Split(gormTag, ";") {
		if strings.TrimSpace(part) == "primaryKey" {
			return true
		}
	}
	return false
}

func isMissingSQLiteTableError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(strings.ToLower(err.Error()), "no such table:")
}

func normalizePage(page int) int {
	if page <= 0 {
		return 1
	}

	return page
}

func normalizePerPage(perPage int) int {
	if perPage <= 0 {
		return 20
	}

	return perPage
}
