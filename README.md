# jb - command line jira board

jb displays your jira board in command line.

It's still buggy, but good enough my usage. Pull requests are welcome.

####Features
- Navigate between issues
- Change issue status (column)
- Open issues with a browser

####Installation
- Clone the repo
- Build it with `go build jb.go`
- Run the executable to get your example config
- Create the configuration and enjoy!

####Todo
- Make commenting on issues possible
- Display issue details in a window

####Thanks to
- [go-jira](https://github.com/andygrunwald/go-jira)
- [gocui](https://github.com/jroimartin/gocui)
- [viper](https://github.com/spf13/viper)