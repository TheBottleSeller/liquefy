package main

import (
    "fmt"
    "flag"
    "os"
    "bufio"

    "golang.org/x/crypto/ssh"
    log "github.com/Sirupsen/logrus"

    "bargain/liquefy/db"
    "bargain/liquefy/cloudprovider"
)

func main() {
    dbIp := flag.String("dbIp", "", "IP of the DB")
    resourceId := flag.Int("resourceId", 0, "Resource ip")
    flag.Parse()

    if *dbIp == "" {
        panic("Provide a valid db ip")
    }

    if *resourceId == 0 {
        panic("Provide a valid resource ip")
    }

    if err := db.Connect(*dbIp); err != nil {
        panic(err)
    }

    resource, err := db.Resources().Get(uint(*resourceId))
    if err != nil {
        panic("Cannot get resource ")
    }

    user, err := db.Users().Get(resource.OwnerId)
    if err != nil {
        panic("Cannot get user")
    }

    // Get SSH private key to use
    awsAccount, err := db.AwsAccounts().Get(user.AwsAccountID)
    if err != nil {
        panic("Failed to setup mesos on resource %d")
    }

    region := cloudprovider.AzToRegion(resource.AwsAvailabilityZone)
    keyString := awsAccount.GetSshPrivateKey(region)
    privateKey, err := ssh.ParsePrivateKey([]byte(keyString))
    if err != nil {
        panic("Failed parsing private key")
    }

    // Setup ssh session
    config := &ssh.ClientConfig{
        User: "ubuntu",
        Auth: []ssh.AuthMethod{
            ssh.PublicKeys(privateKey),
        },
    }
    sshIp := fmt.Sprintf("%s:22", resource.IP)

    log.Infof("Creating ssh connection to resource: %s", sshIp)
    client, err := ssh.Dial("tcp", sshIp, config)
    if err != nil {
        fmt.Println(err)
        panic("Failed to create ssh connection")
    }
    defer client.Close()

    log.Debugf("Creating ssh session to resource")
    session, err := client.NewSession()
    if err != nil {
        panic("Failed to create ssh session")
    }
    defer session.Close()

    // Set IO
    session.Stdout = os.Stdout
    session.Stderr = os.Stderr
    in, _ := session.StdinPipe()

    // Set up terminal modes
    modes := ssh.TerminalModes{
        ssh.ECHO:          0,     // disable echoing
        ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
        ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
    }

    // Request pseudo terminal
    if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
        log.Fatalf("request for pseudo terminal failed: %s", err)
    }

    // Start remote shell
    if err := session.Shell(); err != nil {
        log.Fatalf("failed to start shell: %s", err)
    }

    // Accepting commands
    for {
        reader := bufio.NewReader(os.Stdin)
        str, _ := reader.ReadString('\n')
        fmt.Fprint(in, str)
    }
}