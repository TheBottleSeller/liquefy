package models

import (
    "bytes"
    "fmt"
    log "github.com/Sirupsen/logrus"
)

type LQError struct {
    Message string
    //Code    int      `json:"error_code,omitempty"`
    Cause   error    `json:"cause,omitempty"`
}

func NewError(message string, cause error) (LQError) {
    e := LQError{ message, cause}
    return e
}

func NewErrorf(cause error, formatMessage string, params ...interface{}) (LQError) {
    e := LQError{ fmt.Sprintf(formatMessage, params...), cause }
    return e
}


func (lqe LQError) Error() string {
    var buffer bytes.Buffer

    buffer.WriteString(lqe.Message)
    buffer.WriteString(" :: ")

    if lqe.Cause != nil {
        buffer.WriteString("Cause : ")
        buffer.WriteString(lqe.Cause.Error())
        buffer.WriteString("\n")
    }

    output := buffer.String()
    log.Error(output)

    return output
}
