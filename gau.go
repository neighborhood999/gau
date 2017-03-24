package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/briandowns/spinner"

	pb "gopkg.in/cheggaaa/pb.v1"
)

// Atom :
type Atom struct {
	currentVersion string
	latestVersion  string
}

const releasesPage = "https://github.com/atom/atom/releases/latest"
const tempFolder = "/tmp/latest"

var atom Atom
var gihub = "https://github.com/atom/atom/releases/download/v"

var help = flag.Bool("help", false, "How to use")
var latest = flag.Bool("latest", false, "Latest stable version")
var update = flag.Bool("update", false, "Update atom editor")

func init() {
	flag.Parse()
}

func main() {
	switch {
	case *help:
		fmt.Print(
			"USAGE:\n",
			"  gau command",
			"\n",
			"COMMANDS:\n",
			"  --help How to use gau for updating atom\n",
			"  --latest Latest stable version\n",
			"  --update Update to latest stable version",
		)
	case *latest:
		checkUpdate := atom.currentStatus()
		if checkUpdate == 1 {
			fmt.Println("Running `gau --update` for the update.")
		}
	case *update:
		checkUpdate := atom.currentStatus()

		if checkUpdate == 1 {
			downloadStatus := make(chan string)
			installStatus := make(chan bool)

			fmt.Println("Downloading .deb file...")
			go downloadAtom(downloadStatus)
			fmt.Printf("\n%s\n", <-downloadStatus)

			s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)

			go install(installStatus)

			time.Sleep(10 * time.Millisecond)
			s.Start()

			if !<-installStatus {
				s.FinalMSG = "Check above Error message！"
				s.Stop()
				os.Exit(0)
			}

			s.FinalMSG = "Complete！Successful installed."
			s.Stop()
			os.Exit(0)
		}

		os.Exit(0)
	default:
		fmt.Printf(
			"Your Atom editor version is: %s, use --help getting more information",
			atom.currentVer(),
		)
	}
}

func (atom *Atom) currentStatus() int {
	if atom.currentVersion == "" || atom.latestVersion == "" {
		atom.currentVer()
		atom.getLatestStableVersion()
	}

	if atom.currentVersion == atom.latestVersion {
		fmt.Println("Your Atom Editor is latest! 😉")
		return 0
	}

	fmt.Printf(
		"Your atom version is: %s, the latest stable version is: %s.\n\n",
		atom.currentVersion,
		atom.latestVersion,
	)

	return 1
}

func (atom *Atom) currentVer() string {
	cmd := exec.Command("atom", "--version")
	stdout, err := cmd.Output()

	if err != nil {
		log.Fatal(err)
	}

	subMatched := regexHelperFunc(`^Atom\s+:\s+(\d+\.\d+\.\d+(?:-\w+\d+)?)`, stdout)
	atom.currentVersion = subMatched[1]

	return atom.currentVersion
}

func (atom *Atom) getLatestStableVersion() string {
	resp, err := http.Get(releasesPage)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	dest, err := os.Create("/tmp/latest")
	if err != nil {
		log.Fatalln(err)
	}
	defer dest.Close()
	io.Copy(dest, resp.Body)

	cmd := exec.Command("/bin/sh", "-c", `cat /tmp/latest | grep -o -E 'href="([^"#]+)atom-amd64.deb"' | cut -d'"' -f2 | sort | uniq`)
	stdout, err := cmd.Output()

	if err != nil {
		log.Fatal(err)
	}

	subMatched := regexHelperFunc(`\d+.\d+.\d+`, stdout)
	atom.latestVersion = subMatched[0]

	return atom.latestVersion
}

func regexHelperFunc(regex string, stdout []byte) []string {
	re := regexp.MustCompile(regex)
	result := re.FindStringSubmatch(string(stdout))

	return result
}

func downloadAtom(status chan string) {
	if atom.latestVersion == "" {
		atom.getLatestStableVersion()
	}

	latest := gihub + atom.latestVersion + "/atom-amd64.deb"

	resp, err := http.Get(latest)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Server return non-200 status: %v\n", resp.Status)
		return
	}

	dest, err := os.Create("/tmp/atom-amd64.deb")
	if err != nil {
		fmt.Printf("Can't create %s: %v\n", "/tmp/atom-amd64.deb", err)
		return
	}
	defer dest.Close()

	bar := pb.New(int(resp.ContentLength)).SetUnits(pb.U_BYTES)
	bar.ShowSpeed = true
	bar.Start()

	reader := bar.NewProxyReader(resp.Body)

	io.Copy(dest, reader)
	bar.Finish()

	status <- "Download finished！"
}

func install(status chan bool) {
	cmd := exec.Command("/bin/sh", "-c", "sudo dpkg -i /tmp/atom-amd64.deb")
	cmd.Stderr = os.Stdout
	out, err := cmd.Output()
	isSuccess := cmd.ProcessState.Success()

	if err != nil {
		fmt.Println(err)
		status <- isSuccess
	}

	fmt.Println(string(out))
	status <- isSuccess
}
