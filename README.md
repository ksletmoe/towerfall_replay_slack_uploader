# towerfall_replay_slack_uploader
A small application written in Go that monitors the Towerfall Ascension replay directory and posts new ones to a specified Slack channel.

## Building
The only external dependency this program has is [go-sqlite3](https://github.com/mattn/go-sqlite3). You should be able to simply install it by running

    go get github.com/mattn/go-sqlite3
Other than that, copy this project into your $GOROOT (either by cloning this repository or by running `$ go get github.com/ksletmoe-elemental/towerfall_replay_slack_uploader`) and run `go build towerfall_replay_slack_uploader.go` from within the project root.

## Running
Copy the build binary and the `towerfall_replay_slack_uploader_conf.json` file into a directory of your choice. Edit `towerfall_replay_slack_uploader_conf.json`, and set correct values for `ReplayDirectoryPath`, `AuthToken`, and `ChannelID` (Please note: this is the channel ID, not name).

See [OS X Towerfall Replays Directory](http://steamcommunity.com/app/251470/discussions/0/540743212975369309/), [Windows Towerfall Replays Directory](https://steamcommunity.com/app/251470/discussions/0/558751812957913795/), [Slack Web API Authentication Tokens](https://api.slack.com/web), and [Slack Channel](https://api.slack.com/types/channel) for more information about what to put in the configuration fields.

Once your configuration file is updated, run the towerfall_replay_slack_uploader binary. The application will post each replay in the directory once (continuing to do so as new ones appear), but will not post a replay more than once, even if the program is restarted.
