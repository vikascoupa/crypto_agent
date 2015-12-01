package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/gin-gonic/contrib/newrelic"
	"github.com/gin-gonic/gin"
	"gopkg.in/airbrake/gobrake.v2"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
)

var (
	// The flag package provides command line configuration options
	// You can see the options using the command line option --help which shows the descriptions below
	configurationFlag = flag.String("configuration-path", "conf.json", "Loads configuration file")
	maxStackTraceSize = 4096
	key               = []byte("this is a symmetric key for test")
)

func encrypt(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	return ciphertext, nil
}

func decrypt(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(text) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)
	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Configuration values for the JSON config file
type Configuration struct {
	Dsn                string
	DbName             string
	BindAddress        string
	MaxConnections     string
	NewRelicLicenseKey string
	AirbrakeProjectID  string
	AirbrakeProjectKey string
	Verbose            string
}

var (
	// route path constants
	encryptPath = "/encrypt"
	decryptPath = "/decrypt"
	echoPath    = "/echo"
)

var airbrake *gobrake.Notifier

func ginErrorHandler(message string, err error, c *gin.Context, printStack bool, sendAirbrake bool) {
	logger := log.New(gin.DefaultWriter, "", log.LstdFlags)
	logger.Printf("%s error:%v", message, err)
	if printStack {
		trace := make([]byte, maxStackTraceSize)
		runtime.Stack(trace, false)
		logger.Printf("stack trace--\n%s\n--", trace)
	}
	if sendAirbrake {
		airbrake.Notify(fmt.Errorf("%s error:%v", message, err), c.Request)
		defer airbrake.Flush()
	}
	c.AbortWithError(http.StatusInternalServerError, err)
}

type Encryption struct {
	Encr_id     string `json:"encr_id"`
	Cipher_text string `json:"cipher_text"`
}

func encryptContext() func(*gin.Context) {
	return func(c *gin.Context) {
		encr := make([]Encryption, 1, 15)

		encr[0].Encr_id = "encryptionId"
		//c.JSON(http.StatusOK, encr)
		cleartext := c.Query("cleartext")

		result, err := encrypt(key, []byte(cleartext))
		if err != nil {
			ginErrorHandler("encrypt failed!", err, c, true, true)
			return
		}

		b := base64.StdEncoding.EncodeToString(result)
		c.JSON(http.StatusOK, b)
	}
}

func decryptContext() func(*gin.Context) {
	return func(c *gin.Context) {
		ciphertext := c.Query("ciphertext")

		data, err := base64.StdEncoding.DecodeString(ciphertext)

		//result, err := decrypt(key, []byte(data))
		result, err := decrypt(key, data)
		if err != nil {
			ginErrorHandler("decrypt failed!", err, c, true, true)
			return
		}
		c.JSON(http.StatusOK, string(result))
	}
}

func newEncryptContext() func(*gin.Context) {
	return func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"status": "success"})
	}
}

func echoContext() func(*gin.Context) {
	return func(c *gin.Context) {
		echo := c.Query("echo")
		c.JSON(http.StatusOK, echo)
	}
}

func buildRoutes(r *gin.Engine) {
	v1 := r.Group("/v1")
	{
		v1.GET(encryptPath, encryptContext())
		v1.POST(encryptPath, newEncryptContext())
		v1.GET(decryptPath, decryptContext())
		v1.GET(echoPath, echoContext())
	}
}

func loadConfiguration() (*Configuration, error) {
	file, err := os.Open(*configurationFlag)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	decoder := json.NewDecoder(file)
	var configuration Configuration
	err = decoder.Decode(&configuration)
	if err != nil {
		fmt.Println("error:", err)
		return nil, err
	}
	return &configuration, nil
}

func airbrakeRecovery(airbrake *gobrake.Notifier) gin.HandlerFunc {
	logger := log.New(gin.DefaultWriter, "", log.LstdFlags)
	return func(c *gin.Context) {
		defer func() {
			if rval := recover(); rval != nil {
				rvalStr := fmt.Sprint(rval)
				logger.Printf("recovering for error:%s from uri:%s", rvalStr, c.Request.URL)
				ginErrorHandler("Recovery", errors.New(rvalStr), c, true, true)
			}
			defer airbrake.Flush()
		}()
		c.Next()
	}
}

func main() {
	flag.Parse()
	conf, err := loadConfiguration()
	if err != nil {
		log.Fatal(err)
		return
	}
	verbose, err := strconv.ParseBool(conf.Verbose)
	if err != nil {
		log.Fatal(err)
		return
	}
	airbrakeProjectID, err := strconv.ParseInt(conf.AirbrakeProjectID, 10, 64)
	if err != nil {
		log.Fatal(err)
		return
	}
	airbrake = gobrake.NewNotifier(airbrakeProjectID, conf.AirbrakeProjectKey)
	r := gin.New()
	r.Use(airbrakeRecovery(airbrake)) // Use airbrakeRecovery as early as possible
	r.Use(newrelic.NewRelic(conf.NewRelicLicenseKey, conf.DbName, verbose))
	r.Use(gin.Logger())
	buildRoutes(r)
	r.Run(conf.BindAddress) // listen and serve
}
