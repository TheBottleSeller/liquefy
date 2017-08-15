package models

import (
    "strings"
    "reflect"
)

func AZtoRegion(az string) string {
    return az[:len(az) - 1]
}

// Turns "us_east_1" -> "UsEast1"
func getRegionIdentifier(s string) string {
    sTemp := strings.Replace(s, "-", " ", 2)
    sTemp = strings.Title(sTemp)
    return strings.Replace(sTemp, " ", "", 2)
}

// Turns "us_east_1a" -> "UsEast1A"
func getAzIdentifier(s string) string {
    sTemp := strings.Replace(s, "-", " ", 2)
    sTemp = strings.Title(sTemp)
    sTemp = strings.Replace(sTemp, " ", "", 2)
    az := sTemp[0:len(sTemp) - 1]
    zone := sTemp[len(sTemp) - 1:]
    return az + strings.ToUpper(zone)
}

func (account *AwsAccount) SetVpcId(region string, vpcId string) {
    regionIdentifier := getRegionIdentifier(region)
    mutable := reflect.ValueOf(account).Elem()
    f := mutable.FieldByName("AwsVpcId" + regionIdentifier)
    f.Set(reflect.ValueOf(&vpcId))
}

func (account *AwsAccount) GetVpcId(region string) string {
    regionIdentifier := getRegionIdentifier(region)
    mutable := reflect.ValueOf(account).Elem()
    f := mutable.FieldByName("AwsVpcId" + regionIdentifier)
    val := f.Elem().String()
    if val == "<invalid Value>" {
        return ""
    } else {
        return val
    }
}

func (account *AwsAccount) SetSshPrivateKey(region string, sshPrivateKey string) {
    regionIdentifier := getRegionIdentifier(region)
    mutable := reflect.ValueOf(account).Elem()
    f := mutable.FieldByName("AwsSshPrivateKey" + regionIdentifier)
    f.Set(reflect.ValueOf(&sshPrivateKey))
}

func (account *AwsAccount) GetSshPrivateKey(region string) string {
    regionIdentifier := getRegionIdentifier(region)
    mutable := reflect.ValueOf(account).Elem()
    f := mutable.FieldByName("AwsSshPrivateKey" + regionIdentifier)
    val := f.Elem().String()
    if val == "<invalid Value>" {
        return ""
    } else {
        return val
    }
}

func (account *AwsAccount) SetSubnetId(az string, subnetId string) {
    azIdentifier := getAzIdentifier(az)
    mutable := reflect.ValueOf(account).Elem()
    f := mutable.FieldByName("AwsSubnetId" + azIdentifier)
    f.Set(reflect.ValueOf(&subnetId))
}

func (account *AwsAccount) GetSubnetId(az string) string {
    azIdentifier := getAzIdentifier(az)
    mutable := reflect.ValueOf(account).Elem()
    f := mutable.FieldByName("AwsSubnetId" + azIdentifier)
    val := f.Elem().String()
    if val == "<invalid Value>" {
        return ""
    } else {
        return val
    }
}

func (account *AwsAccount) SetSecurityGroupId(region string, sgId string) {
    regionIdentifier := getRegionIdentifier(region)
    mutable := reflect.ValueOf(account).Elem()
    f := mutable.FieldByName("AwsSecurityGroupId" + regionIdentifier)
    f.Set(reflect.ValueOf(&sgId))
}

func (account *AwsAccount) GetSecurityGroupId(region string) string {
    regionIdentifier := getRegionIdentifier(region)
    mutable := reflect.ValueOf(account).Elem()
    f := mutable.FieldByName("AwsSecurityGroupId" + regionIdentifier)
    val := f.Elem().String()
    if val == "<invalid Value>" {
        return ""
    } else {
        return val
    }
}

func (account *AwsAccount) SetSecurityGroupName(region string, sgName string) {
    regionIdentifier := getRegionIdentifier(region)
    mutable := reflect.ValueOf(account).Elem()
    f := mutable.FieldByName("AwsSecurityGroupName" + regionIdentifier)
    f.Set(reflect.ValueOf(&sgName))
}

func (account *AwsAccount) GetSecurityGroupName(region string) string {
    regionIdentifier := getRegionIdentifier(region)
    mutable := reflect.ValueOf(account).Elem()
    f := mutable.FieldByName("AwsSecurityGroupName" + regionIdentifier)
    val := f.Elem().String()
    if val == "<invalid Value>" {
        return ""
    } else {
        return val
    }
}
