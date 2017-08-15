package logging

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	elastigo "github.com/mattbaird/elastigo/lib"
	"os"
	"time"
)

const (
	VERSION = "1.0.0"
)

type ElasticrusHook struct {
	host            string
	port            string
	baseIndex       string
	indexType       string
	timestampFormat string
	esCon           *elastigo.Conn
}

func NewElasticHook(host, port, baseIndex, indexType, timestampFormat string) *ElasticrusHook {
	hook := &ElasticrusHook{
		host:            host,
		port:            port,
		baseIndex:       baseIndex,
		indexType:       indexType,
		timestampFormat: timestampFormat,
	}

	return hook
}

func (hook *ElasticrusHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}

func (hook *ElasticrusHook) Fire(sourceEntry *logrus.Entry) error {
	hook.esCon = elastigo.NewConn()
	defer hook.esCon.Close()

	hook.esCon.Domain = hook.host
	hook.esCon.Port = hook.port

	transEntry := logrus.NewEntry(&logrus.Logger{ Formatter: &logrus.JSONFormatter{}})
	transEntry.Data = sourceEntry.Data
	transEntry.Time = sourceEntry.Time
	transEntry.Level = sourceEntry.Level
	transEntry.Message = sourceEntry.Message

	logMessage, err := transEntry.String()

	//TODO: Fix the index to be better have more time stamps
	_, err = hook.esCon.Index(hook.getIndexWithDate(), hook.indexType, "", nil, string(logMessage))

	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read entry, %v", err)
		return err
	}

	return err
}

func (hook *ElasticrusHook) getIndexWithDate() string {
	currentDate := time.Now().Format("2006.01.02")
	return hook.baseIndex + "-" + currentDate
}
