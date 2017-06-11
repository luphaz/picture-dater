package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/unicode/norm"
)

var (
	logger        = log.New(os.Stdout, "", log.LstdFlags)
	src           = flag.String("src", ".", "Specify the source directory to scan image from")
	dest          = flag.String("dest", "ready", "Specify the destination directory to save image to")
	ext           = flag.String("ext", ".jpg", "File extension used to filter images")
	loc           = flag.String("location", "", "Location to add to the date, will be formatted using --format if not empty; otherwise only 'date' will be displayed")
	format        = flag.String("format", "%v, %v", "Format used to combine date & location")
	textSize      = flag.Int("text-size", 100, "Text size used to display annotation text, in point")
	bottomMargin  = flag.Int("bottom-margin", 30, "Bottom margin to adjust text centering, in px")
	font          = flag.String("font", "Arial", "Font used to write annotation")
	useGoroutine  = flag.Bool("use-goroutine", false, "Are goroutine used?")
	maxGoroutines = flag.Int("max-goroutines", 10, "Maximum number of goroutines to run in parallel, equivalent to maximum images to resize in parallel")
	help          = flag.Bool("help", false, "Display default flag")
	guard         chan struct{}
)

var formats = map[string]string{
	"January":   "janvier",
	"February":  "février",
	"March":     "mars",
	"April":     "avril",
	"May":       "mai",
	"June":      "juin",
	"July":      "juillet",
	"August":    "août",
	"September": "septembre",
	"October":   "octobre",
	"November":  "novembre",
	"December":  "décembre",
}

func main() {
	// First retrieve CLI flags values to check if help is necessary, just in case
	flag.Parse()

	// In case of a parallel run, let's initialize a guard to avoid running too much concurrent goroutines
	if *useGoroutine {
		guard = make(chan struct{}, *maxGoroutines)
	}

	// Ok, for now help is just defaults
	// TODO : add app presentation & a little bit more description, just because you  can
	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	// We expect the user to use completion, but, as an engineer, you should never trust user inputs!
	// Go is a try & ask for forgiveness language, don't check first that the directory exists
	files, err := ioutil.ReadDir(*src)
	if err != nil {
		logger.Fatalln(err)
	}

	// For more convenience, we will create the destination folder for the user if it doesn't exists
	destFolderPath := path.Join(*src, *dest)
	if _, err := os.Stat(destFolderPath); err != nil && os.IsNotExist(err) {
		err := os.Mkdir(destFolderPath, os.ModePerm)
		if err != nil {
			logger.Fatalln("An error occured when trying to create the destination folder : ", err)
		}
	}

	var wg sync.WaitGroup
	actOnFiles(&wg, files, *loc, *src, destFolderPath)
	wg.Wait()
}

func actOnFiles(wg *sync.WaitGroup, files []os.FileInfo, location string, source string, destination string) {
	// Now, we'll iterate through all files inside a dedicated folder
	// TODO : add a flag to allow a recursive scan of directories, because we all should be lazy, and it's cheap
	logger.Printf("%v files to process on directory %v", len(files), source)
	for _, file := range files {
		if *useGoroutine {
			guard <- struct{}{} // would block if guard channel is already filled

			wg.Add(1)

			go func(sy *sync.WaitGroup, f os.FileInfo, loca string, sour string, des string) {
				defer sy.Done()

				actOnFile(wg, f, loca, sour, des)
			}(wg, file, location, source, destination)
		} else {
			actOnFile(wg, file, location, source, destination)
		}
	}
}

func actOnFile(wg *sync.WaitGroup, f os.FileInfo, location string, source string, destination string) {
	if *useGoroutine {
		defer func() {
			<-guard
		}()
	}

	if f.IsDir() {
		if f.Name() != path.Base(destination) {
			newSource := path.Join(source, f.Name())
			// logger.Println("New  source is : ", newSource, "Source is : ", source)
			files, err := ioutil.ReadDir(newSource)
			if err != nil {
				logger.Println("Can't read directory :", newSource)
			} else {
				actOnFiles(wg, files, f.Name(), newSource, destination)
			}
		}
		return
	}

	// User can enter extension like he want, we just want to include what has been entered, sometimes the simpler the better
	if localExt := filepath.Ext(f.Name()); localExt == "" || !strings.Contains(*ext, localExt) {
		logger.Printf("%v excluded, invalid extension [%v] (expect contained in %v)", f.Name(), localExt, *ext)
		return
	}

	// Yeah, a little bit of context, you know, because we can
	logger.Println("Processing image : ", f.Name())

	// // Get raw date as string from exif data with identify from imagemagick
	// exifPictureDate := extractExifInfoFrom(pattern, f)

	// // Convert string to date
	// pictureDate, err := time.Parse("2006:01:02 15:04:05", exifPictureDate)

	// Date is retrieved from file-name
	// TODO : add a flag to specify which kind of parsing should be used (consider several can be chained, and in which order)
	pictureDate, err := time.Parse("2006-01-02_15-04-05-pola.jpg", f.Name())
	if err != nil {
		logger.Println("Invalid date for file : ", f.Name(), err)
		return
	}

	// Convert date to localized date with only relevant data,  like omitting hour minutes & seconds
	var displayedDate string
	if pictureDate.Day() == 1 {
		displayedDate = fmt.Sprintf("1er %v", localizeDate(pictureDate, "January 2006"))
	} else {
		displayedDate = localizeDate(pictureDate, "02 January 2006")
	}

	// Inject localized date into image with imagemagick
	var annotation string
	if location != "" {
		annotation = fmt.Sprintf(*format, location, displayedDate)
	} else {
		annotation = displayedDate
	}
	annotateImageWith(source, f, destination, annotation, *textSize, *bottomMargin)
}

func localizeDate(date time.Time, layout string) string {
	monthKey := date.Format("January")
	return strings.Replace(date.Format(layout), monthKey, formats[monthKey], -1)
}

func extractExifInfoFrom(rootDirectory string, image os.FileInfo) string {
	out, err := exec.Command("identify", "-format", "%[EXIF:DateTimeOriginal]", path.Join(rootDirectory, image.Name())).CombinedOutput()
	if err != nil {
		logger.Println("Call to identify return following error : ", err, out)
	}
	return string(out)
}

func annotateImageWith(rootDirectory string, image os.FileInfo, destination string, annotation string, textInPointSize int, bottomMargin int) {
	bottomHeightInPixel := 350
	textInPixel := (float32(textInPointSize) / 0.75) / 2
	textPositionFromBottom := (float32(bottomHeightInPixel) / 2) - (textInPixel / 2) - float32(bottomMargin)
	annotateFormat := fmt.Sprintf("+0+%d", int32(textPositionFromBottom))
	cmd := exec.Command("convert", path.Join(rootDirectory, image.Name()), "-font", *font, "-pointsize", strconv.Itoa(textInPointSize), "-fill", "black", "-gravity", "south", "-annotate", annotateFormat, normalizeUtf8Style(annotation), path.Join(destination, image.Name()))
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Println("Call to convert return following error :", err, string(out))
	}
}

// https://blog.golang.org/normalization
// Because MacOS is a elitist system, of course, it uses "the other norm"
// 99.8% of UTF-8 web text are normalized using NFC form, MacOS file system uses NFD...
func normalizeUtf8Style(value string) string {
	return string(norm.NFC.Bytes([]byte(value)))
}
