package main
import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)
import qrcode "github.com/skip2/go-qrcode"

type File struct {
	Filename string
	IsFile bool
	Path string
}

func main() {
	r := gin.Default()
	r.LoadHTMLGlob(filepath.Join("templates/*.html"))
	r.Static("/static", "./static")
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/files")
	})
	r.GET("/files/*fileName", files)
	r.GET("/settings", settings)
	r.POST("/qr", getQrCode)
	r.GET("/about", func(c *gin.Context) {
		c.HTML(http.StatusOK, "about.html", nil)
	})
	r.GET("/help", func(c *gin.Context) {
		c.HTML(http.StatusOK, "help.html", nil)
	})
	r.GET("/opendir", openDir)
	server := &http.Server{
		Addr:    ":8080",
		Handler:  r,
	}
	r.GET("/halt", func(c *gin.Context) {
		// Haven't finish
		ctx, cancel := context.WithTimeout(context.Background(), 0*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})

	r.Run() // listen and serve on 0.0.0.0:8080
}

func files(c *gin.Context) {
	baseDir, _ := os.Getwd()
	fileDir := filepath.Join(baseDir, "files")
	osInfo := runtime.GOOS
	fileName := c.Param("fileName")
	if fileName == "/" {
		is_admin := authenticate(c)
		localFileDir := fileDir
		if osInfo == "windows" {
			localFileDir = strings.Replace(localFileDir, "/", "\\", -1)
		}
		dirs, err := getDirInfo(localFileDir)
		if err != nil {
			c.String(http.StatusBadRequest, "Sorry")
		} else {
			username, err2 := getUsername()
			if err2 != nil {
				c.String(http.StatusNotFound, "Not found setting file")
			} else {
				c.HTML(http.StatusOK, "index.html", gin.H{"cwd": localFileDir, "is_admin": is_admin, "dirs": dirs, "username": username})
			}
		}
	} else {
		localFileDir := filepath.Join(fileDir, fileName)
		if osInfo == "windows" {
			localFileDir = strings.Replace(localFileDir, "/", "\\", -1)
		}
		isFile := IsFile(localFileDir)
		if isFile == 1 {
			//send file
			c.Writer.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName[strings.LastIndex(fileName, "/"):]))
			c.Writer.Header().Add("Content-Type", "application/octet-stream")
			c.File(localFileDir)
		} else if isFile == 0 {
			// show directory
			dirs, err := getDirInfo(localFileDir)
			if err != nil {
				c.String(http.StatusBadRequest, "Sorry")
			}
			username, err2 := getUsername()
			if err2 != nil {
				c.String(http.StatusNotFound, "Not found setting file")
			} else {
				c.HTML(http.StatusOK, "index.html", gin.H{"cwd": localFileDir, "is_admin": true, "dirs": dirs, "username": username})
			}
		} else {
			// Not found
			c.String(http.StatusNotFound, "Sorry, no such files")
		}
	}
}

func settings(c *gin.Context) {
	if authenticate(c) {
		username, err := getUsername()
		if err != nil {
			c.String(http.StatusNotFound, "Not found setting file")
		} else {
			c.HTML(http.StatusOK, "settings.html", gin.H{"username": username})
		}
	} else {
		c.String(http.StatusForbidden, "U r not the host")
	}
}

func openDir(c *gin.Context) {
	osInfo := runtime.GOOS
	if authenticate(c) {
		dir := c.Query("dir")
		if osInfo == "windows" {
			// Tested
			cmd := exec.Command("cmd.exe", "/c", "explorer", dir)
			cmd.Run()
		} else if osInfo == "darwin" {
			// Haven't test
			cmd := exec.Command("open", dir)
			cmd.Run()
		} else {
			// Haven't test
			cmd := exec.Command("nautilus", dir)
			cmd.Run()
		}
		//if err != nil {
		//	c.String(http.StatusBadRequest, "failed open")
		//} else {
		//	c.String(http.StatusOK, "ok")
		//}
	}
}

func getQrCode (c *gin.Context) {
	url := c.PostForm("filepath")
	result := md5.Sum([]byte(url))
	fileName := fmt.Sprintf("%x.png", result)
	imagePath := filepath.Join("./static/QRcode", fileName)
	_, err := os.Create(imagePath)
	if err != nil {
		c.String(http.StatusBadRequest, "Failed to generate QRcode")
	} else {
		err2 := qrcode.WriteFile(url, qrcode.Medium, 256, imagePath)
		if err2 != nil {
			c.String(http.StatusBadRequest, "Failed to generate QRcode")
		} else {
			c.HTML(http.StatusOK, "qrcode.html", gin.H{"imagePath": strings.Replace(imagePath, "\\", "/", -1)})
		}
	}
}

func getUsername() (string, interface{}){
	content, err := ioutil.ReadFile("./config.ini")
	return string(content), err
}

func authenticate(c *gin.Context) bool{
	// check request IP with host ip
	// TODO
	// Don't find the way to get user's ip
	return true
}

func getDirInfo(path string) ([]File, interface{}) {
	baseIndex := strings.Index(path, "\\files")
	ip := LocalIp()
	basePath := path[baseIndex: len(path)]
	fileSlice := make([]File, 0)
	f, err := os.OpenFile(path, os.O_RDONLY, os.ModeDir)
	if err != nil {
		return fileSlice, err
	}
	defer f.Close()
	fileInfo, _ := f.Readdir(-1)
	for _, info := range fileInfo {
		fileSlice = append(fileSlice, File{Filename: info.Name(), IsFile: !info.IsDir(), Path: "http://" + ip + ":8080" + strings.Replace(filepath.Join(basePath, info.Name()), "\\", "/", -1)})
	}
	return fileSlice, err
}

func LocalIp() string {
	addrList, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println(err)
	}
	var ip string = "localhost"
	for _, address := range addrList {
		if ipNet, ok := address.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ip = ipNet.IP.String()
			}
		}
	}
	return ip
}

func IsFile(f string) int {
	fi, e := os.Stat(f)
	if e != nil {
		return -1
	}
	if fi.IsDir() {
		return 0
	}
	return 1
}

func sendFile(fileName string, mimeType string, asAttachment bool,
	attachmentFileName string, addETags bool, cacheTimeOut int,
	conditional bool, lastModified int) {

}

func gracefulExitWeb(server *http.Server) {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	sig := <-ch

	fmt.Println("got a signal", sig)
	now := time.Now()
	cxt, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	err := server.Shutdown(cxt)
	if err != nil{
		fmt.Println("err", err)
	}
	fmt.Println("------exited--------", time.Since(now))
}






