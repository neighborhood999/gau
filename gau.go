package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/briandowns/spinner"
	"github.com/docopt/docopt-go"
	pb "gopkg.in/cheggaaa/pb.v1"
)

// Atom representation current and the latest editor version
type Atom struct {
	version       string
	latestVersion string
}

const fileName = "atom-amd64.deb"
const latestReleases = "https://github.com/atom/atom/releases/latest"
const githubReleasesPage = "https://github.com/atom/atom/releases/download/v"
const usage = `
Usage:
	gau --latest | Get the latest atom version
	gau --upgrade | Download and install atom editor
	gau --help | Help
`

var atom Atom

func main() {
	args, err := docopt.Parse(usage, nil, true, "0.0.5", false)
	if err != nil {
		log.Fatalf("error: %s", err)
	}

	switch {
	case args["--latest"].(bool):
		if status := atom.checkLatestVersion(); status {
			log.Println("Executing `gau --upgrade` for upgrade.")
		}

		os.Exit(0)
	case args["--upgrade"].(bool):
		if status := atom.checkLatestVersion(); status {
			downloadStatus := make(chan string)
			installStatus := make(chan bool)

			log.Println("Downloading .deb file...")
			go downloadAtom(downloadStatus)
			log.Printf("\n%s\n", <-downloadStatus)

			s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)

			go install(installStatus)

			time.Sleep(10 * time.Millisecond)
			s.Start()

			if !<-installStatus {
				s.FinalMSG = "🚨 Check error message！"
				s.Stop()
				os.Exit(1)
			}

			s.FinalMSG = "Success installation ！🎉"
			s.Stop()
			os.Exit(0)
		}

		os.Exit(0)
	}
}

func (atom *Atom) checkLatestVersion() bool {
	atom.getVersion()
	atom.getLatestStableVersion()

	if atom.version == atom.latestVersion {
		log.Printf(
			"Your Atom is the latest \033[1m%s\033[m version!  ✔️",
			atom.latestVersion,
		)
		return false
	}

	log.Printf(
		"Your version: \033[1m%s\033[m, the latest stable version: \033[1m%s\033[m.\n",
		atom.version,
		atom.latestVersion,
	)

	return true
}

func (atom *Atom) getVersion() {
	cmd := exec.Command("atom", "--version")
	stdout, err := cmd.Output()

	if err != nil {
		log.Fatalln(err)
	}

	subMatched := regexHelperFunc(`^Atom\s+:\s+(\d+\.\d+\.\d+(?:-\w+\d+)?)`, stdout)
	atom.version = subMatched[1]
}

func (atom *Atom) getLatestStableVersion() {
	response, err := http.Get(latestReleases)
	if err != nil {
		log.Fatalln(err)
	}
	defer response.Body.Close()

	dest, err := os.Create("/tmp/atom-release-page")
	if err != nil {
		log.Fatalln(err)
	}
	defer dest.Close()
	io.Copy(dest, response.Body)

	cmd := exec.Command(
		"/bin/sh",
		"-c",
		`cat /tmp/atom-release-page | grep -o -E 'href="([^"#]+)atom-amd64.deb"' | cut -d'"' -f2 | sort | uniq`,
	)
	stdout, err := cmd.Output()

	if err != nil {
		log.Fatal(err)
	}

	subMatched := regexHelperFunc(`\d+.\d+.\d+`, stdout)
	atom.latestVersion = subMatched[0]
}

func regexHelperFunc(regex string, stdout []byte) []string {
	re := regexp.MustCompile(regex)
	result := re.FindStringSubmatch(string(stdout))

	return result
}

func downloadAtom(status chan string) {
	atom.getLatestStableVersion()

	latest := githubReleasesPage + atom.latestVersion + "/" + fileName

	response, err := http.Get(latest)
	if err != nil {
		log.Fatalln(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		log.Printf("Server return non-200 status: %v\n", response.Status)
		os.Exit(1)
	}

	dest, err := os.Create("/tmp/atom-amd64.deb")
	if err != nil {
		log.Printf("Can't create %s: %v\n", "/tmp/atom-amd64.deb", err)
		os.Exit(1)
	}
	defer dest.Close()

	bar := pb.New(int(response.ContentLength)).SetUnits(pb.U_BYTES)
	bar.ShowSpeed = true
	bar.Start()

	reader := bar.NewProxyReader(response.Body)

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
		log.Println(err)
		status <- isSuccess
	}

	log.Println(string(out))
	status <- isSuccess
}
