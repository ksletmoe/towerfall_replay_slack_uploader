package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const SLACK_API_URL string = "https://slack.com/api/files.upload"
const DB_PATH string = "./posted_replays.sqlite.db"
const CONF_PATH string = "./towerfall_replay_slack_uploader_conf.json"
const CHECK_INTERVAL_SECONDS time.Duration = time.Duration(30)

func main() {
	success := true
	if config, err := readConfig(CONF_PATH); err != nil {
		log.Printf("Error reading the configuration at '%s': %s", CONF_PATH, err)
		success = false
	} else {
		if err = initializeDbIfNotExist(DB_PATH); err != nil {
			log.Printf("Error initializing the database at '%s': %s", DB_PATH, err)
			success = false
		} else {
			if err = watchReplayDir(DB_PATH, config); err != nil {
				log.Printf("Error watching the replay directory: %s", err)
				success = false
			}
		}
	}

	if !success {
		os.Exit(1)
	}
}

func watchReplayDir(dbPath string, config *Config) error {
	if db, err := sql.Open("sqlite3", dbPath); err != nil {
		return err
	} else {
		log.Printf("Watching directory '%s' for replays to upload...", config.ReplayDirectoryPath)
		for {
			if err := checkAndUploadReplays(db, config); err != nil {
				return err
			}
			time.Sleep(CHECK_INTERVAL_SECONDS * time.Second)
		}
	}
}

func checkAndUploadReplays(db *sql.DB, config *Config) error {
	if replayPaths, err := filepath.Glob(filepath.Join(config.ReplayDirectoryPath, "*.gif")); err != nil {
		return err
	} else {
		for replayPathsIdx := range replayPaths {

			replayFilePath := replayPaths[replayPathsIdx]
			replayName := filepath.Base(replayFilePath)

			if replayUploaded, uploadedCheckError := checkReplayAlreadyUploaded(replayName, db); uploadedCheckError != nil {
				return uploadedCheckError
			} else {
				if !replayUploaded {
					if err := uploadReplay(replayFilePath, config); err != nil {
						return err
					} else {
						log.Printf("Uploaded replay '%s'", replayFilePath)
						if err := recordReplayWasUploaded(replayName, db); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

func uploadReplay(replayFilePath string, config *Config) error {
	log.Printf("Uploading replay '%s'", replayFilePath)

	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	defer bodyWriter.Close()

	replayFileName := filepath.Base(replayFilePath)

	fileWriter, err := bodyWriter.CreateFormFile("file", replayFileName)
	if err != nil {
		return err
	}

	fh, err := os.Open(replayFilePath)
	if err != nil {
		return err
	}
	defer fh.Close()

	if _, err := io.Copy(fileWriter, fh); err != nil {
		return err
	}

	// add the auth token
	tokenField, err := bodyWriter.CreateFormField("token")
	if err != nil {
		return err
	}
	tokenField.Write([]byte(config.AuthToken))

	// add the filename
	filenameField, err := bodyWriter.CreateFormField("filename")
	if err != nil {
		return err
	}
	filenameField.Write([]byte(replayFileName))

	// add the channel to post this to
	channelField, err := bodyWriter.CreateFormField("channels")
	if err != nil {
		return err
	}
	channelField.Write([]byte(config.ChannelID))

	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	resp, err := http.Post(SLACK_API_URL, contentType, bodyBuf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("Error uploading replay '%s': %d", replayFilePath, resp.StatusCode))
	}

	if err := checkResponseOk(resp.Body); err != nil {
		return err
	}

	return nil
}

func checkResponseOk(responseBody io.ReadCloser) error {
	bodyJsonString, err := ioutil.ReadAll(responseBody)
	if err != nil {
		return err
	}

	var responseBodyObj ResponseBody

	err = json.Unmarshal([]byte(bodyJsonString), &responseBodyObj)
	if err != nil {
		log.Printf("Error parsing JSON response body: %s", bodyJsonString)
		return err
	}

	if responseBodyObj.Ok != true {
		return errors.New(fmt.Sprintf("Error uploading replay: %s", responseBodyObj.Error))
	}

	return nil
}

func checkReplayAlreadyUploaded(fileName string, db *sql.DB) (bool, error) {
	stmnt, err := db.Prepare("SELECT COUNT(*) FROM posted_replays WHERE replay_file_name = ?")
	defer stmnt.Close()

	if err != nil {
		log.Printf("Error preparing database statement: %s", err)
		return false, err
	}

	var count int
	err = stmnt.QueryRow(fileName).Scan(&count)

	if err != nil {
		return false, err
	} else {
		return count != 0, nil
	}
}

func recordReplayWasUploaded(replayFileName string, db *sql.DB) error {
	stmnt, err := db.Prepare("INSERT INTO posted_replays VALUES(?);")
	defer stmnt.Close()

	if err != nil {
		return errors.New(fmt.Sprintf("Error recording that replay '%s' was uploaded: %s", replayFileName, err))
	} else {
		if _, err := stmnt.Exec(replayFileName); err != nil {
			return errors.New(fmt.Sprintf("Error recording that replay '%s' was uploaded: %s", replayFileName, err))
		}
	}

	return nil
}

func initializeDbIfNotExist(dbPath string) error {
	if !fileExists(dbPath) {
		db, err := sql.Open("sqlite3", dbPath)
		defer db.Close()

		if err != nil {
			return err
		} else {
			_, err := db.Exec("CREATE TABLE posted_replays(replay_file_name varchar(512));")
			if err != nil {
				return err
			}
		}

	}

	return nil
}

type ResponseBody struct {
	Ok    bool
	Error string
}

type Config struct {
	ReplayDirectoryPath string
	AuthToken           string
	ChannelID           string
}

func readConfig(confFilePath string) (*Config, error) {
	if confBytes, err := ioutil.ReadFile(confFilePath); err != nil {
		return nil, err
	} else {
		conf := new(Config)
		err = json.Unmarshal(confBytes, conf)

		if err != nil {
			return nil, err
		} else {
			return conf, nil
		}
	}
}

func fileExists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	} else {
		return true
	}
}
