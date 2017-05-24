package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"

	"os/exec"
	"sync"
	"time"

	"github.com/alexivanenko/nex_ssh/config"
	"golang.org/x/crypto/ssh"
)

func main() {
	config.Log("Run nEx SSH")

	//Local Machine Part
	runNex()
	defer killNetExtender()

	fmt.Println("SSH Connect: First Attempt")
	conn, err := sshConnect()

	if err == nil {
		execLocalPullPush()
	} else {
		//If nEx still not connected sleep, wait and try to connect to server again
		time.Sleep(time.Second * time.Duration(config.Int("net_extender", "run_timeout")))
		fmt.Println("SSH Connect: Second Attempt")
		conn, err = sshConnect()

		if err == nil {
			execLocalPullPush()
		}
	}

	//Server Side Part
	if err == nil {
		var pullList []string
		var fullPath string

		pullList = config.Strings("server_pull_list")
		pullResult := make(chan string, 10)

		for _, pullDir := range pullList {
			fullPath = config.String("git_dirs", "server_web_root") + pullDir

			go func(conn *ssh.Client, dir string) {
				pullResult <- execServerPull(conn, dir)
			}(conn, fullPath)
		}

		for i := 0; i < len(pullList); i++ {
			select {
			case res := <-pullResult:
				config.Log(res)
			}
		}

	} else {
		fmt.Println(fmt.Errorf("Failed to Dial: %s", err))
	}
}

//runNex runs netExtender process in the background and returns PID
func runNex() {
	cmdString := "netExtender -u " + config.String("net_extender", "username") +
		" -p " + config.String("net_extender", "password") +
		" -d " + config.String("net_extender", "domain") +
		" " + config.String("net_extender", "server") + ":" +
		config.String("net_extender", "port") +
		" &"

	cmd := exec.Command("sh", "-c", cmdString)
	err := cmd.Start()

	if err != nil {
		fmt.Println(fmt.Errorf("Exec Background Cmd Error: %s", err))
	}

	cmd.Wait()

	//Sleep for few seconds seconds and wait while NetExtender init
	//Use this bad way because didn't resolve the problem with
	//running the nEx process, reading stdout and go down to
	//next line of code
	time.Sleep(time.Second * time.Duration(config.Int("net_extender", "run_timeout")))
}

//killNetExtender kills netExtender process
func killNetExtender() {
	commands := []string{"pkill netExtender"}
	runCommands(commands)
}

//execLocalPullPush executes expect script for running `git pull`
// and `git push` commands for a given repository
func execLocalPullPush() {
	scriptPath := config.GetRootDir() + "/sh/" + config.String("git_dirs", "local_update_script")
	commands := []string{"expect " + scriptPath}
	runCommands(commands)
}

//runCommands execute a list of shell commands
func runCommands(commands []string) {
	wg := new(sync.WaitGroup)

	for _, str := range commands {
		wg.Add(1)
		go execCmd(str, wg)
	}

	wg.Wait()
}

//execCmd executes shell command
func execCmd(cmd string, wg *sync.WaitGroup) {
	out, err := exec.Command("sh", "-c", cmd).Output()

	if err != nil {
		fmt.Println(fmt.Errorf("Exec Cmd Error: %s", err))
	}

	fmt.Printf("%s", out)
	wg.Done()
}

//execServerPull using SSH connection session runs `git pull`
// command for a given repository
func execServerPull(conn *ssh.Client, dir string) string {
	var result string
	session, err := conn.NewSession()
	defer session.Close()

	if err == nil {
		var stdoutBuf, stderrBuf bytes.Buffer
		session.Stdout = &stdoutBuf
		session.Stderr = &stderrBuf

		err := session.Run("cd " + dir + "; git pull")

		if err != nil {
			fmt.Println(fmt.Errorf("%s (%s)", err, dir))
		} else {
			result = dir + " output - StdOut: " + stdoutBuf.String()

			if stderrBuf.String() != "" {
				result += " StdErr: " + stderrBuf.String()
			}
		}
	} else {
		fmt.Println(fmt.Errorf("Failed to open Session: %s", err))
	}

	return result
}

//sshConnect connects to the server through SSH
func sshConnect() (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User: config.String("ssh", "user"),
		Auth: []ssh.AuthMethod{publicKeyFile(os.Getenv("HOME") + "/.ssh/id_rsa")},
	}

	return ssh.Dial("tcp", config.String("ssh", "host")+":"+config.String("ssh", "port"), sshConfig)
}

func publicKeyFile(file string) ssh.AuthMethod {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil
	}

	return ssh.PublicKeys(key)
}
