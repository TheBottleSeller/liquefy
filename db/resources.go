package db

import (
    log "github.com/Sirupsen/logrus"
    lq "bargain/liquefy/models"
    "github.com/jinzhu/gorm"
    "time"
    "fmt"
)

type ResourcesTable interface {
    CreateInTx(*gorm.DB, *lq.ResourceInstance) error
    Get(resourceID uint) (*lq.ResourceInstance, error)
    Update(resourceId uint, resource *lq.ResourceInstance) error

    GetNewResources() ([]*lq.ResourceInstance, error)
    GetTerminatedResourceIdsWithAssignedJobs() ([]uint, error)

    GetUsersResources(userID uint) ([]*lq.ResourceInstance, error)
    GetUsersProvisionedResources(userID uint) ([]*lq.ResourceInstance, error)

    GetAllProvisionedResources() ([]*lq.ResourceInstance, error)
    GetAllProvisionedOrRunningResources() ([]*lq.ResourceInstance, error)

    // Status changing
    SetStatus(resourceId uint, status, msg string) error

    // Update metadata
    SetInstanceId(resourceId uint, status string) error
    SetLaunchTime(resourceId uint, launchTime int64) error
    SetIP(resourceId uint, ip string) error

    GetRunningUserTerminatedResources() ([]*lq.ResourceInstance, error)
    MarkUserTerminated(resourceId uint) error

    Delete(resourceId uint) error
}

type resourcesTable struct {}

func Resources() ResourcesTable {
    return &resourcesTable{}
}

func (table *resourcesTable) CreateInTx(tx *gorm.DB, resource *lq.ResourceInstance) error {
    resource.Status = lq.ResourceStatusNew

    if err := tx.Create(resource).Error; err != nil {
        return lq.NewErrorf(err, "Failed creating resource instance")
    }

    if err := trackStatus(tx, resource.ID, resource.Status, ""); err != nil {
        return lq.NewErrorf(err, "Failed creating initial resource event")
    }

    return nil
}

func (table *resourcesTable) Get(resourceId uint) (*lq.ResourceInstance, error) {
    var resource lq.ResourceInstance
    query := db.Find(&resource, resourceId)
    if query.Error != nil {
        return &resource, lq.NewError(fmt.Sprintf("Failed fetching resource %d", resourceId), query.Error)
    }
    return &resource, nil
}

func (table *resourcesTable) GetNewResources() ([]*lq.ResourceInstance, error) {
    resources := []*lq.ResourceInstance{}
    query := db.Where("status = ?", lq.ResourceStatusNew).Find(&resources)
    if query.Error != nil {
        return resources, lq.NewError("Failed fetching all new resources", query.Error)
    }
    return resources, nil
}

func (table *resourcesTable) GetTerminatedResourceIdsWithAssignedJobs() ([]uint, error) {
    resourceIds := []uint{}

    rows, err := db.Raw(fmt.Sprintf("SELECT id FROM resource_instance WHERE " +
        "status = '%s' OR status = '%s' AND " +
        "(SELECT COUNT(*) FROM container_job WHERE instance_id = resource_instance.id) > 0",
        lq.ResourceStatusDeprovisioning, lq.ResourceStatusDeprovisioned)).Rows()
    if err != nil {
        return resourceIds, lq.NewErrorf(err, "Failed getting all terminated resources with assigned jobs")
    }
    for rows.Next() {
        var id uint
        err = rows.Scan(&id)
        if err != nil {
            return resourceIds, lq.NewErrorf(err, "Failed getting all terminated resources with assigned jobs by row")
        }
        resourceIds = append(resourceIds, id)
    }

    return resourceIds, nil
}

func (table *resourcesTable)  GetUsersResources(userID uint) ([]*lq.ResourceInstance, error) {
    resources := []*lq.ResourceInstance{}
    query := db.Where("owner_id = ?", userID).Find(&resources)
    if query.Error != nil {
        return resources, lq.NewError(fmt.Sprintf("Failed fetching all resources for user %d",
            userID), query.Error)
    }
    return resources, nil
}

func (table *resourcesTable) GetUsersProvisionedResources(userID uint) ([]*lq.ResourceInstance, error) {
    activeResources := []*lq.ResourceInstance{}
    query := db.Where("(status = ? OR status = ?) AND owner_id = ?",
        lq.ResourceStatusProvisioned, lq.ResourceStatusRunning, userID).Find(&activeResources)
    if query.Error != nil {
        return activeResources, lq.NewError(fmt.Sprintf("Failed fetching active resources for user %d",
            userID), query.Error)
    }
    return activeResources, nil
}

func (table *resourcesTable) GetAllProvisionedResources() ([]*lq.ResourceInstance, error) {
    activeResources := []*lq.ResourceInstance{}
    query := db.Where("status = ?", lq.ResourceStatusProvisioned).Find(&activeResources)
    if query.Error != nil {
        return activeResources, lq.NewError("Failed fetching all provisioned resources", query.Error)
    }
    return activeResources, query.Error
}

func (table *resourcesTable) GetAllProvisionedOrRunningResources() ([]*lq.ResourceInstance, error) {
    activeResources := []*lq.ResourceInstance{}
    query := db.Where("status = ? OR status = ?",
        lq.ResourceStatusProvisioned, lq.ResourceStatusRunning, ).Find(&activeResources)
    if query.Error != nil {
        return activeResources, lq.NewError("Failed fetching all provisioned resources", query.Error)
    }
    return activeResources, query.Error
}

func (table *resourcesTable) GetRunningUserTerminatedResources() ([]*lq.ResourceInstance, error) {
    userTerminatedResources := []*lq.ResourceInstance{}
    query := db.Where("user_terminated = true AND status = ?", lq.ResourceStatusRunning).Find(&userTerminatedResources)
    if query.Error != nil {
        return userTerminatedResources, lq.NewError("Failed fetching all running, user terminated resources", query.Error)
    }
    return userTerminatedResources, query.Error
}

func (table *resourcesTable) Update(resourceId uint, resource *lq.ResourceInstance) error {
    resource.ID = resourceId
    query := db.Model(&lq.ResourceInstance{}).Update(resource)
    if query.Error != nil {
        return lq.NewError(fmt.Sprintf("Failed updating resource %d", resourceId), query.Error)
    }
    return nil
}

func (table *resourcesTable) SetStatus(resourceId uint, status, msg string) (err error) {
    var resource lq.ResourceInstance
    tx := db.Begin()
    defer TxCommitOrRollback(tx, &err, "Failed setting resource %d status to %s ", resourceId, status)

    if err = tx.Find(&resource, resourceId).Error; err != nil {
        return
    }

    err = table.setStatusTx(tx, resourceId, resource.Status, status, msg)

    return
}

func (table *resourcesTable) setStatusTx(tx *gorm.DB, resourceId uint, currentStatus, newStatus, msg string) error {
    if ! table.validateStateTransition(currentStatus, newStatus) {
        return fmt.Errorf("Invalid resource state transition: %s to %s", currentStatus, newStatus)
    }

    if err := tx.Exec(fmt.Sprintf(
    "UPDATE resource_instance SET status = '%s' WHERE id = %d", newStatus, resourceId)).Error; err != nil {
        return err
    }

    return trackStatus(tx, resourceId, newStatus, msg)
}

func trackStatus(tx *gorm.DB, resourceId uint, status, msg string) (err error) {
    // truncate message if necessary
    maxMsgLen := 1024
    if len(msg) > maxMsgLen {
        msg = msg[:maxMsgLen]
    }
    event := &lq.ResourceEvent{
        InstanceID: resourceId,
        Status: status,
        Time: time.Now().UTC().UnixNano(),
        Msg: msg,
    }

    if err = tx.Table("resource_events").Create(event).Error; err != nil {
        err = lq.NewError("Failed creating resource event", err)
    }
    return
}

func (table *resourcesTable) SetLaunchTime(resourceId uint, launchTime int64) error {
    query := db.Model(&lq.ResourceInstance{}).Where("id = ?", resourceId).UpdateColumn("launch_time", launchTime)
    if query.Error != nil {
        log.Error(query.Error)
    }
    return query.Error
}

func (table *resourcesTable) SetInstanceId(resourceId uint, instanceId string) error {
    query := db.Model(&lq.ResourceInstance{}).Where("id = ?", resourceId).UpdateColumn("aws_instance_id", instanceId)
    if query.Error != nil {
        log.Error(query.Error)
    }
    return query.Error
}

func (table *resourcesTable) SetIP(resourceId uint, ip string) error {
    query := db.Model(&lq.ResourceInstance{}).Where("id = ?", resourceId).UpdateColumn("ip", ip)
    if query.Error != nil {
        log.Error(query.Error)
    }
    return query.Error
}

func (table *resourcesTable) MarkUserTerminated(resourceId uint) error {
    sql := fmt.Sprintf("UPDATE resource_instance SET user_terminated = true WHERE id = %d", resourceId)
    query := db.Exec(sql)
    if query.Error != nil {
        return lq.NewErrorf(query.Error, "Failed updating resource %d as user terminated", resourceId)
    }
    return nil
}

func (table *resourcesTable) Delete(resourceId uint) (err error) {
    query := db.Where("id = ?", resourceId).Delete(&lq.ResourceInstance{})
    if query.Error != nil {
        log.Error(query.Error)
    }
    return query.Error
}

func (table *resourcesTable) validateStateTransition(currentState, newState string) bool {
    validTransitions := make(map[string][]string)

    validTransitions[lq.ResourceStatusNew] = []string{
        lq.ResourceProvisioning,
    }

    validTransitions[lq.ResourceProvisioning] = []string{
        lq.ResourceSpotBidding,
        lq.ResourceStatusDeprovisioning, // If provisioning fails at this step, resource will be sent for deprovisioning
    }

    validTransitions[lq.ResourceSpotBidding] = []string{
        lq.ResourceSpotBidAccepted,
        lq.ResourceStatusDeprovisioning, // If provisioning fails at this step, resource will be sent for deprovisioning
    }

    validTransitions[lq.ResourceSpotBidAccepted] = []string{
        lq.ResourceStatusProvisioned,
        lq.ResourceStatusDeprovisioning, // If provisioning fails at this step, resource will be sent for deprovisioning
    }

    validTransitions[lq.ResourceStatusProvisioned] = []string{
        lq.ResourceStatusRunning,
        lq.ResourceStatusDeprovisioning, // If provisioning fails at this step, resource will be sent for deprovisioning
    }

    validTransitions[lq.ResourceStatusRunning] = []string{
        lq.ResourceStatusRunning,
        lq.ResourceStatusDeprovisioning,
    }

    validTransitions[lq.ResourceStatusDeprovisioning] = []string{
        lq.ResourceStatusDeprovisioned,
    }

    for _, validState := range validTransitions[currentState] {
        if validState == newState {
            return true
        }
    }
    return false
}
