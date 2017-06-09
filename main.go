package main

import (
	"bufio"
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
	"time"
)

var (
	logger       = log.New(os.Stdout, "", log.LstdFlags)
	src          = flag.String("src", ".", "Specify the source directory to scan image from")
	dest         = flag.String("dest", "ready", "Specify the destination directory to save image to")
	ext          = flag.String("ext", ".jpg", "File extension used to filter images")
	loc          = flag.String("location", "", "Location to add to the date, will be formatted using --format if not empty; otherwise only 'date' will be displayed")
	format       = flag.String("format", "%v, %v", "Format used to combine date & location")
	textSize     = flag.Int("text-size", 100, "Text size used to display annotation text, in point")
	bottomMargin = flag.Int("bottom-margin", 30, "Bottom margin to adjust text centering, in px")
	font         = flag.String("font", "Arial", "Font used to write annotation")
	help         = flag.Bool("help", false, "Display default flag")
)

func main() {
	// First retrieve CLI flags values to check if help is necessary, just in case
	flag.Parse()

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

	// Now, we'll iterate through all files inside a dedicated folder
	// TODO : add a flag to allow a recursive scan of directories, because we all should be lazy, and it's cheap
	logger.Printf("%v files to process on directory %v", len(files), *src)
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		// User can enter extension like he want, we just want to include what has been entered, sometimes the simpler the better
		if localExt := filepath.Ext(f.Name()); !strings.Contains(*ext, localExt) {
			logger.Printf("%v excluded, invalid extension %v (expect contained in %v)", f.Name(), localExt, *ext)
			continue
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
			continue
		}

		// Convert date to localized date with only relevant data,  like omitting hour minutes & seconds
		var displayedDate string
		if pictureDate.Day() == 1 {
			displayedDate = fmt.Sprintf("1er %v", pictureDate.Format("janvier 2006"))
		} else {
			displayedDate = pictureDate.Format("02 janvier 2006")
		}

		// Inject localized date into image with imagemagicke
		var annotation string
		if *loc != "" {
			annotation = fmt.Sprintf(*format, *loc, displayedDate)
		} else {
			annotation = displayedDate
		}
		annotateImageWith(*src, f, *dest, annotation, *textSize, *bottomMargin, *font)
	}
}

func askToUser(question string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(question)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func extractExifInfoFrom(rootDirectory string, image os.FileInfo) string {
	out, err := exec.Command("identify", "-format", "%[EXIF:DateTimeOriginal]", path.Join(rootDirectory, image.Name())).CombinedOutput()
	if err != nil {
		logger.Println("Call to identify return following error : ", err, out)
	}
	return string(out)
}

func annotateImageWith(rootDirectory string, image os.FileInfo, destFolder string, annotation string, textInPointSize int, bottomMargin int, font string) {
	bottomHeightInPixel := 350
	textInPixel := (float32(textInPointSize) / 0.75) / 2
	textPositionFromBottom := (float32(bottomHeightInPixel) / 2) - (textInPixel / 2) - float32(bottomMargin)
	annotateFormat := fmt.Sprintf("+0+%d", int32(textPositionFromBottom))
	// logger.Println(annotateFormat)
	cmd := exec.Command("convert", path.Join(rootDirectory, image.Name()), "-font", font, "-pointsize", strconv.Itoa(textInPointSize), "-fill", "black", "-gravity", "south", "-annotate", annotateFormat, annotation, path.Join(rootDirectory, destFolder, image.Name()))
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Println("Call to convert return following error :", err, string(out))
	}
}
