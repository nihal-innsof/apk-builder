package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"time"
)

const apkDirectory string = "build/app/outputs/flutter-apk"

// Run flutter build command
func buildApk(flag string) {
	var cmd *exec.Cmd
	if flag == "dev" {
		cmd = exec.Command("flutter", "build", "apk", "--debug", "--flavor", flag, "--dart-define", "FLAVOR="+flag)
	} else {
		cmd = exec.Command("flutter", "build", "apk", "--split-per-abi", "--flavor", flag, "--dart-define", "FLAVOR="+flag)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func filterDirectories(source []fs.DirEntry, test func(fs.DirEntry) bool) (ret []fs.DirEntry) {
	for _, item := range source {
		if test(item) {
			ret = append(ret, item)
		}
	}
	return ret
}

func compressApks(flavor string) bool {
	directoryContents, err := os.ReadDir(apkDirectory)
	if err != nil {
		fmt.Println("Build directory not found. Please run flutter build apk first.")
		panic(err)
	}
	directoryContents = filterDirectories(directoryContents, func(item fs.DirEntry) bool {
		isMatch, err := regexp.MatchString(".*"+flavor+".*release.*.apk$", item.Name())
		if err != nil {
			panic(err)
		}
		return isMatch
	})
	if len(directoryContents) == 0 {
		fmt.Println("No file generated for flavor:", flavor)
		return false
	}
	fmt.Println("Compressing APKs...")
	zipFile, err := os.Create("build-apk.zip")
	if err != nil {
		panic(err)
	}
	zipWriter := zip.NewWriter(zipFile)
	if err != nil {
		panic(err)
	}
	defer zipWriter.Close()
	for _, item := range directoryContents {
		if item.IsDir() {
			continue
		}
		file, err := os.Open(apkDirectory + "/" + item.Name())
		if err != nil {
			panic(err)
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			panic(err)
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			panic(err)
		}
		header.Name = item.Name()
		header.Method = zip.Deflate
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			panic(err)
		}
		_, err = io.Copy(writer, file)
		if err != nil {
			panic(err)
		}
	}
	fmt.Println("APKs compressed successfully!")
	return true
}

func uploadFile(flavor string) {
	var cmd *exec.Cmd
	done := make(chan bool)
	fmt.Println("Uploading file")
	go showSpinner(done)
	if flavor == "dev" {
		cmd = exec.Command("curl", "--upload-file", "./build/app/outputs/flutter-apk/app-dev-debug.apk", "https://free.keep.sh")
	} else {
		cmd = exec.Command("curl", "--upload-file", "./build-apk.zip", "https://free.keep.sh")
	}
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	done <- true
	if err != nil {
		fmt.Println("\nCurl command failed:", err)
		os.Exit(1)
	}
	fmt.Println("File upload completed successfuly")
}

func showSpinner(done <-chan bool) {
	// Define the spinner characters
	spinner := `|/-\`

	i := 0
	for {
		select {
		case <-done:
			return
		default:
			// Print the current spinner character
			fmt.Printf("\b%c", spinner[i])
			i = (i + 1) % len(spinner)
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func main() {
	flavorFlag := flag.String("flavor", "dev", "Flavor to build")
	helpFlag := flag.Bool("help", false, "Show help")
	flag.Parse()
	if *helpFlag {
		flag.Usage()
		return
	}
	buildApk(*flavorFlag)
	if *flavorFlag != "dev" {
		compressionSuccess := compressApks(*flavorFlag)
		if !compressionSuccess {
			return
		}
	}
	uploadFile(*flavorFlag)
}
