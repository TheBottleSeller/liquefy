package db
import (
    "fmt"
    lq "bargain/liquefy/models"
)

type MesosTable interface {
    CreateTable() error
    DropTable() error

    SetFrameworkId(frameworkId string) error
    GetFrameworkId() (string, error)
}

type mesosTable struct{}

func Mesos() MesosTable {
    return &mesosTable{}
}

func (table *mesosTable) CreateTable() error {
    return db.Exec("CREATE TABLE mesos_info ( framework_id VARCHAR(1024) UNIQUE NOT NULL )").Error
}

func (table *mesosTable) DropTable() error {
    return db.Exec("DROP TABLE mesos_info").Error
}

func (table *mesosTable) GetFrameworkId() (string, error) {
    frameworkId := ""
    rows, err := db.Raw("SELECT framework_id FROM mesos_info").Rows()
    if err != nil {
        return "", lq.NewErrorf(err, "Failed getting framework id")
    }

    // If there is a framework id present, return it
    hasRowWithId := rows.Next()
    if hasRowWithId {
        err = rows.Scan(&frameworkId)
        if err != nil {
            return "", lq.NewErrorf(err, "Failed getting reading framework id row")
        }
    }

    return frameworkId, nil
}

func (table *mesosTable) SetFrameworkId(frameworkId string) error {
    existingId, err := table.GetFrameworkId()
    if err != nil {
        return err
    }

    if existingId != "" {
        return lq.NewErrorf(nil, "Framework id already exists")
    }

    return db.Exec(fmt.Sprintf("INSERT INTO mesos_info VALUES ('%s')", frameworkId)).Error
}