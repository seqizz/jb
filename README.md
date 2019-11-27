# jb - command line JIRA kanban board

jb displays your JIRA kanban board in command line.

![Example image of jb](https://i.imgur.com/USBwqyH.jpg)

It might be still buggy, but usable. Pull requests are welcome.

#### Features

- Navigate between issues
- Change issue status
- Open issues with a browser
- Preview issue details

#### Installation

- Get it with `go get github.com/seqizz/jb`
- Run the executable with -confighelp to get your example config
- Create the configuration and enjoy faster JIRA!

#### Todo

- Make commenting on issues possible
- Make issue preview better
- Make possible to set assignee
- Comment on issues

#### Thanks to

- [go-jira](https://github.com/andygrunwald/go-jira)
- [gocui](https://github.com/jroimartin/gocui)
- [viper](https://github.com/spf13/viper)
- [logrus](https://github.com/sirupsen/logrus)
