package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/aquasecurity/kube-bench/check"
	"github.com/fatih/color"
	"github.com/golang/glog"
)

var (
	// Print colors
	colors = map[check.State]*color.Color{
		check.PASS: color.New(color.FgGreen),
		check.FAIL: color.New(color.FgRed),
		check.WARN: color.New(color.FgYellow),
		check.INFO: color.New(color.FgBlue),
	}
)

func printlnWarn(msg string) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n",
		colors[check.WARN].Sprintf("%s", check.WARN),
		msg,
	)
}

func sprintlnWarn(msg string) string {
	return fmt.Sprintf("[%s] %s",
		colors[check.WARN].Sprintf("%s", check.WARN),
		msg,
	)
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "\n%v\n", err)
	os.Exit(1)
}

func continueWithError(err error, msg string) string {
	if err != nil {
		glog.V(1).Info(err)
	}

	if msg != "" {
		fmt.Fprintf(os.Stderr, "%s\n", msg)
	}

	return ""
}

func cleanIDs(list string) []string {
	list = strings.Trim(list, ",")
	ids := strings.Split(list, ",")

	for _, id := range ids {
		id = strings.Trim(id, " ")
	}

	return ids
}

func verifyConf(confPath ...string) {
	var missing string

	for _, c := range confPath {
		if _, err := os.Stat(c); err != nil && os.IsNotExist(err) {
			continueWithError(err, "")
			missing += c + ", "
		}
	}

	if len(missing) > 0 {
		missing = strings.Trim(missing, ", ")
		printlnWarn(fmt.Sprintf("Missing kubernetes config files: %s", missing))
	}

}

func verifyBin(binPath ...string) {
	var binSlice []string
	var bin string
	var missing string
	var notRunning string

	// Construct proc name for ps(1)
	for _, b := range binPath {
		_, err := exec.LookPath(b)
		bin = bin + "," + b
		binSlice = append(binSlice, b)
		if err != nil {
			missing += b + ", "
			continueWithError(err, "")
		}
	}
	bin = strings.Trim(bin, ",")

	cmd := exec.Command("ps", "-C", bin, "-o", "cmd", "--no-headers")
	out, err := cmd.Output()
	if err != nil {
		continueWithError(fmt.Errorf("%s: %s", cmd.Args, err), "")
	}

	for _, b := range binSlice {
		matched := strings.Contains(string(out), b)

		if !matched {
			notRunning += b + ", "
		}
	}

	if len(missing) > 0 {
		missing = strings.Trim(missing, ", ")
		printlnWarn(fmt.Sprintf("Missing kubernetes binaries: %s", missing))
	}

	if len(notRunning) > 0 {
		notRunning = strings.Trim(notRunning, ", ")
		printlnWarn(fmt.Sprintf("Kubernetes binaries not running: %s", notRunning))
	}
}

func verifyKubeVersion(major string, minor string) {
	// These executables might not be on the user's path.

	_, err := exec.LookPath("kubectl")
	if err != nil {
		continueWithError(err, sprintlnWarn("Kubernetes version check skipped"))
		return
	}

	cmd := exec.Command("kubectl", "version")
	out, err := cmd.Output()
	if err != nil {
		s := fmt.Sprintf("Kubernetes version check skipped with error %v", err)
		continueWithError(err, sprintlnWarn(s))
		return
	}

	msg := checkVersion("Client", string(out), major, minor)
	if msg != "" {
		continueWithError(fmt.Errorf(msg), msg)
	}

	msg = checkVersion("Server", string(out), major, minor)
	if msg != "" {
		continueWithError(fmt.Errorf(msg), msg)
	}
}

var regexVersionMajor = regexp.MustCompile("Major:\"([0-9]+)\"")
var regexVersionMinor = regexp.MustCompile("Minor:\"([0-9]+)\"")

func checkVersion(x string, s string, expMajor string, expMinor string) string {
	regexVersion, err := regexp.Compile(x + " Version: version.Info{(.*)}")
	if err != nil {
		return fmt.Sprintf("Error checking Kubernetes version: %v", err)
	}

	ss := regexVersion.FindString(s)
	major := versionMatch(regexVersionMajor, ss)
	minor := versionMatch(regexVersionMinor, ss)
	if major == "" || minor == "" {
		return fmt.Sprintf("Couldn't find %s version from kubectl output '%s'", x, s)
	}

	if major != expMajor || minor != expMinor {
		return fmt.Sprintf("Unexpected %s version %s.%s", x, major, minor)
	}

	return ""
}

func versionMatch(r *regexp.Regexp, s string) string {
	match := r.FindStringSubmatch(s)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}
