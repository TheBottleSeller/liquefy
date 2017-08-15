package common

import (
    "os"
    "fmt"
)

func ValidateDeployment() {
    env := os.Getenv("ENV")
    if env != "PRODUCTION" && env != "STAGING" && env != "LOCAL" {
        panic(fmt.Sprintf("Invalid value %s for $ENV environment variable. Must be PRODUCTION, STAGING, or LOCAL", env))
    }
}

func IsProductionDeployment() bool {
    return os.Getenv("ENV") == "PRODUCTION"
}

func IsStagingDeployment() bool {
    return os.Getenv("ENV") == "STAGING"
}

func IsLocalDeployment() bool {
    return os.Getenv("ENV") == "LOCAL"
}