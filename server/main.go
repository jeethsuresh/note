package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo"
)

func main() {
	// Echo instance
	e := echo.New()

	// Routes
	e.POST("/upload/:user/:file", upload)
	e.GET("/download/list", downloadList)
	e.GET("/download/:user/:file", downloadFile)

	// Start server
	e.Logger.Fatal(e.Start(":1323"))
}

// upload file, with file overwriting
func upload(c echo.Context) error {
	bodyBytes, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		fmt.Println("Error reading uploaded file: " + err.Error())
	}

	user := c.Param("user")
	filename := c.Param("file")

	newpath := filepath.Join(".", user)
	os.MkdirAll(newpath, os.ModePerm)

	err = os.Remove(newpath + "/" + filename + ".txt")
	if err != nil {
		fmt.Println("file delete err: " + err.Error())
	}
	f, err := os.Create(newpath + "/" + filename + ".txt")
	if err != nil {
		fmt.Println("file create err: " + err.Error())
	}
	_, err = f.Write(bodyBytes)
	if err != nil {
		fmt.Println("file write err: " + err.Error())
	}

	return c.String(http.StatusOK, "Hello, World!")
}

// download a list of all files on the server
func downloadList(c echo.Context) error {
	var files map[string]map[string][]string

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		dir, err := filepath.Abs(filepath.Dir(path))
		if err != nil {
			fmt.Println(err)
		}
		realdirarr := strings.Split(dir, "/")
		realdir := realdirarr[len(realdirarr)-1]

		files["files"][realdir] = append(files["files"][realdir], info.Name())
		return nil
	})
	if err != nil {
		panic(err)
	}

	return c.JSON(http.StatusOK, files)
}

// download a specific file
func downloadFile(c echo.Context) error {
	user := c.Param("user")
	newpath := filepath.Join(".", user)
	filename := c.Param("file")
	filepath := newpath + "/" + filename + ".txt"

	return c.File(filepath)
}
