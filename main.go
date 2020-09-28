package main

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	_ "image/gif"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
)

const (
	appendTag      = "_small"
	maxSizeLarger  = 640
	maxSizeSmaller = 480
)

var (
	hasError = false
	wait     = &sync.WaitGroup{}
)

//never destroy origional image
//always append _small (but before extension) and write in the same directory
//if argument is a directory, create a new directory (X_small) and
//  keep all files with the same name (but make them small)
//only support jpeg and png files for now (maybe bmp?)
//do not recurse
func main() {
	runtime.GOMAXPROCS(3)

	//iterate through arguments, pass each to func to determine what to do with them
	for i, p := range os.Args {
		if i == 0 {
			continue
		}
		wait.Add(1)
		go smallify(p, "", 0)
	}

	wait.Wait()
	if hasError {
		//? wait for key ?
	}
}

func panicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

//
func smallify(fullname, dir string, level int) {
	//1. determine if the file is a file or folder
	//2. If file, kick off func to make small image
	//   If folder, kick off a sub-process for all images, wait for all to return
	defer wait.Done()
	defer func() {
		if err := recover(); err != nil {
			hasError = true
			fmt.Printf("Error: %s\n", err)
		}
	}()

	fullname, err := filepath.Abs(fullname)
	panicOnError(err)
	fullname = filepath.Clean(fullname)

	file, err := os.Open(fullname)
	panicOnError(err)
	defer file.Close()

	st, err := file.Stat()
	panicOnError(err)

	if dir == "" {
		dir = filepath.Dir(fullname)
	}

	if st.IsDir() == false {
		//I'm a file
		smallifyImage(file, dir, st.Name())
	} else {
		//I'm a directory
		if level > 0 {
			//don't drill down into other dirs
			return
		}
		newDir := filepath.Join(dir, st.Name()+appendTag)
		err := os.MkdirAll(newDir, st.Mode())
		panicOnError(err)

		//read all entries at once (-1)
		entries, err := file.Readdir(-1)
		panicOnError(err)
		for i := 0; i < len(entries); i++ {
			e := entries[i]
			if e.IsDir() {
				//don't drill down into other dirs
				continue
			}
			wait.Add(1)
			go smallify(filepath.Join(fullname, e.Name()), newDir, level+1)
		}
	}
}

//pass in an open file, don't close
//this will create a file in dir
func smallifyImage(file *os.File, dir, name string) {
	img, as, err := image.Decode(file)
	panicOnError(err)

	var w writeImage
	newExt := ""

	switch as {
	case "jpeg":
		//jpeg
		w = writeJpeg
		newExt = "jpg"
	case "png", "bmp", "tiff":
		//png
		w = writePng
		newExt = "png"
	default:
		panic("unknown type: " + as)
	}

	ext := filepath.Ext(name)
	justName := name[0 : len(name)-len(ext)]

	//construct the new full file path
	newName := filepath.Join(dir, justName+appendTag+"."+newExt)
	newFile, err := os.Create(newName)
	panicOnError(err)
	defer newFile.Close()

	sz := img.Bounds().Size()

	landscape := sz.X >= sz.Y
	var larger, smaller int
	if landscape {
		larger = sz.X
		smaller = sz.Y
	} else {
		larger = sz.Y
		smaller = sz.X
	}

	scaleA := float64(maxSizeLarger) / float64(larger)
	scaleB := float64(maxSizeSmaller) / float64(smaller)

	scale := scaleA
	if scale > scaleB {
		scale = scaleB
	}

	x := int(float64(larger) * scale)
	y := int(float64(smaller) * scale)

	if !landscape {
		x, y = y, x
	}

	w(img, newFile, image.Point{X: x, Y: y})
}

//must take an image and scale and write img to writer
type writeImage func(img image.Image, w io.Writer, newSize image.Point)

func writeJpeg(img image.Image, w io.Writer, newSize image.Point) {
	newImg := image.NewNRGBA(image.Rectangle{Min: image.Point{0, 0}, Max: newSize})
	draw.BiLinear.Scale(newImg, newImg.Rect, img, img.Bounds(), draw.Over, nil)
	err := jpeg.Encode(w, newImg, nil)
	panicOnError(err)
}

func writePng(img image.Image, w io.Writer, newSize image.Point) {
	newImg := image.NewRGBA(image.Rectangle{Min: image.Point{0, 0}, Max: newSize})
	draw.BiLinear.Scale(newImg, newImg.Rect, img, img.Bounds(), draw.Over, nil)
	err := png.Encode(w, newImg)
	panicOnError(err)
}
