// Application that creates pagesets on a CT worker and uploads it to Google
// Storage.
package main

import (
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/golang/glog"

	"strconv"

	"skia.googlesource.com/buildbot.git/ct/go/util"
	"skia.googlesource.com/buildbot.git/go/common"
)

var (
	workerNum   = flag.Int("worker_num", 1, "The number of this CT worker. It will be in the {1..100} range.")
	pagesetType = flag.String("pageset_type", util.PAGESET_TYPE_MOBILE_10k, "The type of pagesets to create from the Alexa CSV list. Eg: 10k, Mobile10k, All.")
)

func main() {
	common.Init()
	// Create the task file so that the master knows this worker is still busy.
	util.CreateTaskFile(util.ACTIVITY_CREATING_PAGESETS)
	defer util.DeleteTaskFile(util.ACTIVITY_CREATING_PAGESETS)

	// Delete and remake the local pagesets directory.
	pathToPagesets := filepath.Join(util.PagesetsDir, *pagesetType)
	os.RemoveAll(pathToPagesets)
	os.MkdirAll(pathToPagesets, 0700)

	// Get info about the specified pageset type.
	pagesetTypeInfo := util.PagesetTypeToInfo[*pagesetType]
	csvSource := pagesetTypeInfo.CSVSource
	numPages := pagesetTypeInfo.NumPages
	userAgent := pagesetTypeInfo.UserAgent

	// Download the CSV file from Google Storage to a tmp location.
	gs, err := util.NewGsUtil(nil)
	if err != nil {
		glog.Fatal(err)
	}
	respBody, err := gs.GetRemoteFileContents(csvSource)
	if err != nil {
		glog.Fatal(err)
	}
	defer respBody.Close()
	csvFile := filepath.Join(os.TempDir(), filepath.Base(csvSource))
	out, err := os.Create(csvFile)
	if err != nil {
		glog.Fatalf("Unable to create file %s: %s", csvFile, err)
	}
	defer out.Close()
	defer os.Remove(csvFile)
	if _, err = io.Copy(out, respBody); err != nil {
		glog.Fatal(err)
	}

	// Figure out which pagesets this worker should generate.
	numPagesPerSlave := numPages / util.NUM_WORKERS
	startNum := (*workerNum-1)*numPagesPerSlave + 1
	endNum := *workerNum * numPagesPerSlave

	// Construct path to the create_page_set.py python script.
	_, currentFile, _, _ := runtime.Caller(0)
	createPageSetScript := filepath.Join(
		filepath.Dir((filepath.Dir(filepath.Dir(filepath.Dir(currentFile))))),
		"py", "create_page_set.py")

	// Execute the create_page_set.py python script.
	for currNum := startNum; currNum <= endNum; currNum++ {
		cmdArgs := []string{
			createPageSetScript,
			"-s", strconv.Itoa(currNum),
			"-e", strconv.Itoa(currNum),
			"-c", csvFile,
			"-p", *pagesetType,
			"-u", userAgent,
			"-o", pathToPagesets}
		if _, err := exec.Command("python", cmdArgs...).Output(); err != nil {
			glog.Fatal(err)
		}
	}
	// Write timestamp to the pagesets dir.
	util.CreateTimestampFile(pathToPagesets)

	// Upload pagesets dir to Google Storage.
	if err := gs.UploadWorkerArtifacts(util.PAGESETS_DIR_NAME, *pagesetType, *workerNum); err != nil {
		glog.Fatal(err)
	}
}