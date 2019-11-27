package main

import (
    "errors"
    "fmt"
    "os"
    "os/exec"
    "sort"
    "strconv"
    "strings"
    "flag"

    jira "github.com/andygrunwald/go-jira"
    "github.com/jroimartin/gocui"
    "github.com/sirupsen/logrus"
    "github.com/spf13/viper"
)

type issueBox struct {
    view  *gocui.View
    issue jira.Issue
}

type column struct {
    view    *gocui.View
    members []issueBox
}

type activeBox struct {
    columnname       string
    indexno          int
    issuetitle       string
    issueurl         string
    availableActions map[string]string
}

type configItem struct {
    instanceURL    string
    username       string
    password       string
    query          string
    browserCommand string
}

var (
    kanbanlist    = []jira.Issue{}
    kanbanMatrix     = []column{}
    active        = &activeBox{}
    jiraClient    = &jira.Client{}
    configColumns = []string{}
    moveCounter   = 0
    infoText      = "Navigation: Arrow keys  |  Actions Menu: Spacebar  |  Exit: Ctrl-C"
)

var log = logrus.New()

// jiraAction function applies the specified action for the key, which
// is the issue.
func jiraAction(g *gocui.Gui, issue *jira.Issue, action string) {

    if !jiraClient.Authentication.Authenticated() {
        jiraClient = getJiraAuth()
    }

    updateStatus(g, "Action: "+action+". Issue: "+issue.ID)

    realAction := strings.Split(action, "> ")[1]

    actionID := active.availableActions[realAction]

    res, err := jiraClient.Issue.DoTransition(issue.ID, actionID)
    if err != nil {
        updateStatus(g, err.Error())
    }

    menuView, _ := g.View("menu")
    destroyView(g, menuView)

    updateStatus(g, res.Status)

}

// getJiraAuth function creates the initial JIRA authentication cookie
// It uses the pre-read variables jiraUsername and jiraPassword
func getJiraAuth() *jira.Client {

    config := readConfig()

    jiraClient, err := jira.NewClient(nil, config.instanceURL)
    if err != nil {
        panic(err)
    }

    res, err := jiraClient.Authentication.AcquireSessionCookie(
        config.username,
        config.password,
    )
    if err != nil || res == false {
        fmt.Printf("Result: %v\n", res)
        panic(err)
    }

    return jiraClient

}

// executeQuery function executes the JQL query specified in config file
func executeQuery(conf configItem) []jira.Issue {

    if !jiraClient.Authentication.Authenticated() {
        jiraClient = getJiraAuth()
    }

    config := readConfig()

    issuelist, _, err := jiraClient.Issue.Search(config.query, nil)
    if err != nil {
        fmt.Printf("Result: %v\n", err)
        panic(err)
    }

    return issuelist
}

func activateFirstIssue(g *gocui.Gui) {
    for i := range kanbanMatrix {
        if len(kanbanMatrix[i].members) > 0 {
            setCurrentViewOnTop(g, kanbanMatrix[i].members[0].view.Title)
            active.columnname = kanbanMatrix[i].view.Title
            active.issuetitle = kanbanMatrix[i].members[0].view.Title
            active.indexno = 0
            return
        }
    }
}

func setCurrentViewOnTop(g *gocui.Gui, name string) (*gocui.View, error) {
    if _, err := g.SetCurrentView(name); err != nil {
        return nil, err
    }
    return g.SetViewOnTop(name)
}

func getColumn(title string) (*column, error) {

    for i := range kanbanMatrix {
        if kanbanMatrix[i].view.Title == title {
            return &kanbanMatrix[i], nil
        }
    }
    return &column{}, errors.New("Couldn't found column with given title")
}

func registerIssue(box issueBox) bool {

    col, err := getColumn(box.issue.Fields.Status.Name)
    if err != nil {
        return false
    }

    col.members = append(col.members, box)
    return true

}

func getMenuSelection(g *gocui.Gui, v *gocui.View) error {
    var line string
    var err error

    _, cy := v.Cursor()
    if line, err = v.Line(cy); err != nil {
        line = ""
    }

    switch line {
    case "Comment on issue":
        maxX, maxY := g.Size()
        if v, err := g.SetView("msgBox", maxX/2-30, maxY/2, maxX/2+30, maxY/2+2); err != nil {
            if err != gocui.ErrUnknownView {
                return err
            }
            v.Editable = true
            v.Title = "Comment on issue " + active.issuetitle
            setCurrentViewOnTop(g, "msgBox")
        }
        if err := g.SetKeybinding("msgBox", gocui.KeyEsc, gocui.ModNone, destroyView); err != nil {
            log.Panicln(err)
        }
        if err := g.SetKeybinding("msgBox", gocui.KeyCtrlS, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
            updateStatus(g, "Sorry, not implemented yet!")
            destroyView(g, v)
            return nil
        }); err != nil {
            log.Panicln(err)
        }
        updateStatus(g, "Send: Ctrl-S (not implemented yet)  |  Close: Esc")
    case "Preview issue":
        maxX, maxY := g.Size()
        if v, err := g.SetView("previewBox", 5, 3, maxX-5, maxY-3); err != nil {
            if err != gocui.ErrUnknownView {
                return err
            }
            v.Title = "Details of " + active.issuetitle
            v.Autoscroll = false
            v.Wrap = true
            v.Editable = true
            for i := range kanbanMatrix {
                if kanbanMatrix[i].view.Title == active.columnname {
                    issue := kanbanMatrix[i].members[active.indexno].issue
                    lineSlice := strings.SplitN(
                        issue.Fields.Description,
                        "\r\n",
                        -1,
                    )
                    fmt.Fprintln(v, "Reporter: " + issue.Fields.Reporter.DisplayName)
                    if (issue.Fields.Assignee != nil) {
                        fmt.Fprintln(v, "Assignee: " + issue.Fields.Assignee.DisplayName)
                    }
                    if (issue.Fields.Labels != nil) {
                        if len(issue.Fields.Labels) > 0 {
                            fmt.Fprintln( v, "Labels: " + strings.Join(issue.Fields.Labels, ","))
                        }
                    }
                    if (issue.Fields.Priority != nil) {
                        fmt.Fprintln(v, "Priority: "+issue.Fields.Priority.Name)
                    }
                    fmt.Fprint(v, "Description:\n\n")
                    for i := range lineSlice {
                        fmt.Fprintln(v, lineSlice[i])
                    }
                    break
                }
            }
            setCurrentViewOnTop(g, "previewBox")
        }
        if err := g.SetKeybinding("previewBox", gocui.KeyEsc, gocui.ModNone, destroyView); err != nil {
            log.Panicln(err)
        }
        updateStatus(g, "Close: Esc")
    case "Open in browser":
        conf := readConfig()
        issueURL := ""
        col, err := getColumn(active.columnname)
        if err != nil {
            log.Error("Couldn't find active column, something is very wrong.")
        }
        issueURL = conf.instanceURL + "/browse/" + col.members[active.indexno].issue.Key
        cmd := exec.Command(conf.browserCommand, issueURL)
        err = cmd.Start()
        if err != nil {
            log.Fatal(err)
        }
        menuView, _ := g.View("menu")
        destroyView(g, menuView)
    case "":
        log.Debug("User chose nothing, closing the menu, that'll teach the user..")
        menuView, _ := g.View("menu")
        destroyView(g, menuView)
    default:
        col, err := getColumn(active.columnname)
        if err != nil {
            log.Error("Couldn't find active column, something is very wrong.")
        }
        jiraAction(g, &col.members[active.indexno].issue, line)
        refreshBoard(g, v)
    }

    return nil
}

func destroyView(g *gocui.Gui, v *gocui.View) error {
    g.Cursor = false
    if v != nil {
        g.DeleteView(v.Name())
        if _, err := g.View("menu"); err != nil {
            setCurrentViewOnTop(g, active.issuetitle)
            updateStatus(g, infoText)
        } else {
            updateStatus(g, "")
            setCurrentViewOnTop(g, "menu")
        }
    }
    return nil
}

func openMenu(g *gocui.Gui, v *gocui.View) error {

    menuCoord := [4]int{}

    maxX, maxY := g.Size()
    activeIssue := jira.Issue{}
    col, err := getColumn(active.columnname)
    if err != nil {
        log.Error("Couldn't find active column, something is very wrong.")
    }

    xzero, _, _, yone, _ := g.ViewPosition(col.members[active.indexno].view.Title)
    menuCoord = [4]int{xzero, yone, xzero + 32, yone + 14}
    if yone > maxY-14 {
        log.Debug("Menu will not fit, lifting it up a bit")
        menuCoord[1] = menuCoord[1] - 14
        menuCoord[3] = yone
    }
    if xzero + 32 > maxX {
        log.Debug("Menu will not fit, putting it to the left a bit")
        menuCoord[0] = menuCoord[0] - 14
        menuCoord[2] = menuCoord[2] - 14
    }
    activeIssue = col.members[active.indexno].issue

    actionMap := map[string]string{}
    // Let's first check if the issue belongs to us
    availActions, _, err := jiraClient.Issue.GetTransitions(activeIssue.ID)
    if err != nil {
        return err
    }
    for k := range availActions {
        actionMap[availActions[k].To.Name] = availActions[k].ID
    }

    active.availableActions = actionMap

    if v, err := g.SetView(
        "menu", menuCoord[0], menuCoord[1], menuCoord[2], menuCoord[3]); err != nil {
        if err != gocui.ErrUnknownView {
            return err
        }
        v.Editable = false
        v.Highlight = true
        v.Title = v.Name()

        keys := make([]string, 0, len(actionMap))
        for k := range actionMap {
            keys = append(keys, k)
        }
        sort.Strings(keys)
        for _, k := range keys {
            fmt.Fprintln(v, "Send to -> "+k)
        }
        fmt.Fprintln(v, "Open in browser")
        fmt.Fprintln(v, "Preview issue")
        fmt.Fprintln(v, "Comment on issue")
    }

    if err := g.SetKeybinding("menu", gocui.KeyEsc, gocui.ModNone, destroyView); err != nil {
        log.Panicln(err)
    }

    if err := g.SetKeybinding("menu", gocui.KeySpace, gocui.ModNone, destroyView); err != nil {
        log.Panicln(err)
    }

    updateStatus(g, "Run action: Enter  |  Close menu: Esc or Spacebar")
    setCurrentViewOnTop(g, "menu")

    return nil
}

func refreshBoard(g *gocui.Gui, v *gocui.View) error {

    updateStatus(g, "Refreshing board...")

    for i := range kanbanMatrix {
        for m := range kanbanMatrix[i].members {
            g.DeleteKeybindings(kanbanMatrix[i].members[m].view.Title)
            g.DeleteView(kanbanMatrix[i].members[m].view.Title)
        }
    }
    for i := range kanbanMatrix {
        kanbanMatrix[i].members = kanbanMatrix[i].members[:0]
    }

    conf := readConfig()

    for _, issue := range executeQuery(conf) {
        createIssue(g, issue)
    }

    g.SetViewOnTop("statusLine")

    activateFirstIssue(g)

    updateStatus(g, infoText)

    return nil
}

func giveNextIssueCoord(g *gocui.Gui, col column) ([4]int, error) {

    issueCoord := [4]int{}

    if len(col.members) == 0 {
        xzero, yzero, xone, _, err := g.ViewPosition(col.view.Title)
        if err != nil {
            return [4]int{}, err
        }

        issueCoord = [4]int{xzero + 1, yzero + 1, (xzero + (xone - xzero)) - 1, yzero + 5}
    } else {

        // We are getting current member count of the column
        // and determine the position over that
        xzero, _, xone, yone, _ := g.ViewPosition(col.members[len(col.members)-1].view.Title)
        //TODO error catch maybe?
        issueCoord = [4]int{xzero, yone + 1, xone, yone + 6}
    }
    return issueCoord, nil

}

func createIssue(g *gocui.Gui, issue jira.Issue) error {

    correctColumn := column{}
    undefinedColumn := true

    for i := range kanbanMatrix {
        if kanbanMatrix[i].view.Title == issue.Fields.Status.Name {
            correctColumn = kanbanMatrix[i]
            undefinedColumn = false
            break
        }
    }

    if undefinedColumn {
        log.Warn("Issue with undefined column found and skipped.")
        return nil
    }

    validCoords, _ := giveNextIssueCoord(g, correctColumn)
    // TODO error catch maybe?

    if v, err := g.SetView(
        issue.Key, validCoords[0], validCoords[1], validCoords[2], validCoords[3]); err != nil {
        if err != gocui.ErrUnknownView {
            return err
        }
        v.Wrap = true
        componentList := ""
        for i, v := range issue.Fields.Components {
            if i == 0 {
                componentList = componentList + "["
            }
            componentList = componentList + v.Name
            if i == len(issue.Fields.Components)-1 {
                componentList = componentList + "]\n"
            }
        }
        v.Title = issue.Key
        // We will use ANSI color here
        // I hope your terminal is clever enough
        fmt.Fprintln(v, "\x1b[0;34m"+componentList+"\x1b[0m"+issue.Fields.Summary)
        registerIssue(issueBox{view: v, issue: issue})
        if err := g.SetKeybinding(issue.Key, gocui.KeyArrowDown, gocui.ModNone, downHandler); err != nil {
            log.Panicln(err)
        }
        if err := g.SetKeybinding(issue.Key, gocui.KeyArrowUp, gocui.ModNone, upHandler); err != nil {
            log.Panicln(err)
        }
        if err := g.SetKeybinding(issue.Key, gocui.KeyArrowRight, gocui.ModNone, rightHandler); err != nil {
            log.Panicln(err)
        }
        if err := g.SetKeybinding(issue.Key, gocui.KeyArrowLeft, gocui.ModNone, leftHandler); err != nil {
            log.Panicln(err)
        }
        if err := g.SetKeybinding(issue.Key, gocui.KeySpace, gocui.ModNone, openMenu); err != nil {
            log.Panicln(err)
        }
    }

    return nil
}

func moveIssues(g *gocui.Gui, dy int, reset bool) error {
    if reset {
        log.Debug("Got a ordering reset call")
        // Let's see if we're going to bottom or top
        goingToTop := false
        if moveCounter != 0 {
            // We're in moved position, request must be top
            goingToTop = true
        }
        if dy == 1000 {
            // This is right-left arrow, we will reset previous column to top
            goingToTop = true
        }
        if goingToTop {
            log.Debug("View going to the top")
            for i := range kanbanMatrix {
                if kanbanMatrix[i].view.Title == active.columnname {
                    for vi := range kanbanMatrix[i].members {
                        curView := kanbanMatrix[i].members[vi].view.Title
                        // log.Debug("View: " + curView + "MOVE COUNTER: " + strconv.Itoa(moveCounter))
                        c1, c2, c3, c4, _ := g.ViewPosition(curView)
                        if moveCounter > 0 {
                            log.Debug("Coordinates: " +
                                strconv.Itoa(c1) + " " +
                                strconv.Itoa(c2) + " " +
                                strconv.Itoa(c3) + " " +
                                strconv.Itoa(c4))
                            if _, err := g.SetView(curView, c1, c2+6*moveCounter, c3, c4+6*moveCounter); err != nil {
                                return err
                            } else {
                                log.Debug("moveCounter is not positive")
                            }
                        }
                    }
                }
            }
            // Arrangement is done, reset the move counter
            moveCounter = 0
        } else {
            log.Debug("View going to bottom")
            for i := range kanbanMatrix {
                if kanbanMatrix[i].view.Title == active.columnname {
                    issueCount := len(kanbanMatrix[i].members)
                    if issueCount > 5 {
                        for i := 1; i <= issueCount-5; i++ {
                            moveIssues(g, -6, false)
                        }
                    }
                }
            }
        }
        g.SetViewOnTop("statusLine")
        return nil
    }

    for i := range kanbanMatrix {
        if kanbanMatrix[i].view.Title == active.columnname {
            for vi := range kanbanMatrix[i].members {
                curView := kanbanMatrix[i].members[vi].view.Title
                c1, c2, c3, c4, _ := g.ViewPosition(curView)
                if _, err := g.SetView(curView, c1, c2+dy, c3, c4+dy); err != nil {
                    return err
                }
            }
        }
    }
    if dy < 0 {
        moveCounter++
    } else {
        moveCounter--
    }

    g.SetViewOnTop("statusLine")
    return nil
}

func rightHandler(g *gocui.Gui, v *gocui.View) error {
    rightLeftView(g, v, "right")
    return nil
}

func leftHandler(g *gocui.Gui, v *gocui.View) error {
    rightLeftView(g, v, "left")
    return nil
}

func upHandler(g *gocui.Gui, v *gocui.View) error {
    upDownView(g, v, "up")
    return nil
}

func downHandler(g *gocui.Gui, v *gocui.View) error {
    upDownView(g, v, "down")
    return nil
}

func rightLeftView(g *gocui.Gui, v *gocui.View, direction string) error {

    log.Debug(direction + " pressed")

    newView := &gocui.View{}

    newColumnName := active.columnname

    slideNumber := 1
    if direction == "left" {
        slideNumber = -1
    }

    columnFound := false
    for !columnFound {
        for i := range kanbanMatrix {
            if kanbanMatrix[i].view.Title == newColumnName {
                if direction == "right" {
                    if i == len(kanbanMatrix)-1 {
                        newColumnName = kanbanMatrix[0].view.Title
                        if len(kanbanMatrix[0].members) > 0 {
                            columnFound = true
                        }
                        break
                    }
                } else {
                    if i == 0 {
                        newColumnName = kanbanMatrix[len(kanbanMatrix)+slideNumber].view.Title
                        if len(kanbanMatrix[len(kanbanMatrix)+slideNumber].members) > 0 {
                            columnFound = true
                        }
                        break
                    }
                }
                newColumnName = kanbanMatrix[i+slideNumber].view.Title
                if len(kanbanMatrix[i+slideNumber].members) > 0 {
                    columnFound = true
                }
                break
            }
        }
    }

    boxFound := false
    for !boxFound {
        for i := range kanbanMatrix {
            if kanbanMatrix[i].view.Title == newColumnName {
                if len(kanbanMatrix[i].members) == 0 {
                    // Empty column, skip
                    if i == 0 {
                        newColumnName = kanbanMatrix[len(kanbanMatrix)+slideNumber].view.Title
                    } else {
                        newColumnName = kanbanMatrix[i+slideNumber].view.Title
                    }
                    break
                }
                // Not empty
                if len(kanbanMatrix[i].members) > active.indexno && moveCounter == 0 {
                    log.Debug("We didn't move below screen, sliding to same place on new column")
                    active.issuetitle = kanbanMatrix[i].members[active.indexno].view.Title
                    newView = kanbanMatrix[i].members[active.indexno].view
                    log.Debug("Index no: " + strconv.Itoa(active.indexno))
                    log.Debug("Move counter: " + strconv.Itoa(moveCounter))
                } else if moveCounter != 0 {
                    log.Debug("We already moved below screen, sliding to the first issue of new column")
                    active.indexno = 0
                    active.issuetitle = kanbanMatrix[i].members[active.indexno].view.Title
                    newView = kanbanMatrix[i].members[active.indexno].view
                    log.Debug("Index no: " + strconv.Itoa(active.indexno))
                    log.Debug("Move counter: " + strconv.Itoa(moveCounter))
                } else {
                    log.Debug("Next column seems smaller, sliding to the last issue of new column")
                    active.indexno = len(kanbanMatrix[i].members) - 1
                    active.issuetitle = kanbanMatrix[i].members[active.indexno].view.Title
                    newView = kanbanMatrix[i].members[active.indexno].view
                    log.Debug("Index no: " + strconv.Itoa(active.indexno))
                    log.Debug("Move counter: " + strconv.Itoa(moveCounter))
                }
                boxFound = true
                break
            }
        }
    }

    log.Debug("Resetting the view of the column we leave: " + active.columnname)
    moveIssues(g, 1000, true)
    if _, err := setCurrentViewOnTop(g, newView.Title); err != nil {
        return err
    }

    active.columnname = newColumnName
    updateStatus(g, "")
    return nil
}

func upDownView(g *gocui.Gui, v *gocui.View, direction string) error {

    log.Debug(direction + " pressed")

    newView := &gocui.View{}

    slideNumber := 1
    if direction == "up" {
        slideNumber = -1
    }

    for i := range kanbanMatrix {
        if kanbanMatrix[i].view.Title == active.columnname {
            log.Debug("moveCounter pre-ordering value: " + strconv.Itoa(moveCounter))
            if direction == "up" {
                if active.indexno == 0 {
                    log.Debug("Already on top, coordinating move to bottom")
                    active.indexno = len(kanbanMatrix[i].members) - 1
                    active.issuetitle = kanbanMatrix[i].members[active.indexno].view.Title
                    newView = kanbanMatrix[i].members[active.indexno].view
                    moveIssues(g, 0, true)
                    break
                }
            } else {
                if len(kanbanMatrix[i].members) == active.indexno+1 {
                    log.Debug("Already on bottom, coordinating move to top")
                    if len(kanbanMatrix[i].members) == 1 {
                        log.Debug("Wait, there is only one element here.. Doing nothing.")
                        return nil
                    }
                    active.indexno = 0
                    active.issuetitle = kanbanMatrix[i].members[0].view.Title
                    newView = kanbanMatrix[i].members[0].view
                    moveIssues(g, 0, true)
                    break
                }
            }

            log.Debug("We're not on an edge, we'll just slide one " + direction)
            active.indexno = active.indexno + slideNumber
            active.issuetitle = kanbanMatrix[i].members[active.indexno].view.Title
            newView = kanbanMatrix[i].members[active.indexno].view

            columnNeedsToMove := false
            _, maxY := g.Size()
            _, y0, _, y1, _ := g.ViewPosition(newView.Title)
            if (direction == "up") && (y0 < 0) {
                log.Debug("We're close to the edge, will move whole column " + direction)
                columnNeedsToMove = true
            } else if (direction == "down") &&  (y1 > maxY-3) {
                log.Debug("We're close to the edge, will move whole column " + direction)
                columnNeedsToMove = true
            }
            if columnNeedsToMove {
                moveIssues(g, -6*slideNumber, false)
            }
            break
        }
    }

    log.Debug("moveCounter post-ordering value: " + strconv.Itoa(moveCounter))

    if _, err := setCurrentViewOnTop(g, newView.Title); err != nil {
        return err
    }

    updateStatus(g, "")

    return nil
}

func indexOf(element string, data []string) int {
    for k, v := range data {
        if element == v {
            return k
        }
    }
    return -1 //not found.
}
func indexOfView(element string, data []issueBox) int {
    for k, v := range data {
        if element == v.view.Title {
            return k
        }
    }
    return -1 //not found.
}

func drawBoard(g *gocui.Gui) error {
    maxX, realmaxY := g.Size()

    //Bottom area will be used for status line
    maxY := realmaxY - 1

    // This is needed (especially in parallel execution or refresh events),
    // since configColumns will be filled with reading configuration.
    readConfig()

    for _, columnName := range configColumns {

        if v, err := g.SetView(
            columnName,
            maxX/len(configColumns)*indexOf(columnName, configColumns),
            0,
            maxX/len(configColumns)*(indexOf(columnName, configColumns)+1),
            maxY-1,
        ); err != nil {
            if err != gocui.ErrUnknownView {
                return err
            }
            v.Title = v.Name()
            v.Editable = false
            v.Wrap = true

            newcol := column{view: v, members: []issueBox{}}
            kanbanMatrix = append(kanbanMatrix, newcol)

        }

    }

    // Status line
    if v, err := g.SetView(
        "statusLine",
        0,
        realmaxY-2,
        maxX,
        realmaxY,
    ); err != nil {
        if err != gocui.ErrUnknownView {
            return err
        }
        v.Editable = false
        v.Frame = false
        v.BgColor = gocui.ColorMagenta
        fmt.Fprintln(v, "Loading issues...")
    }

    return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
    return gocui.ErrQuit
}

func readConfig() configItem {
    conf := viper.New()
    conf.SetConfigName("jb")        // name of config file (without extension)
    conf.AddConfigPath("/etc/jb")   // path to look for the config file in
    conf.AddConfigPath("$HOME/.jb") // call multiple times to add many search paths
    err := conf.ReadInConfig()      // Find and read the config file
    if err != nil {                 // Handle errors reading the config file
        panic(fmt.Errorf("Fatal error config file: %s \n", err))
    }

    instanceURL := conf.GetString("jira_instance")
    username := conf.GetString("jira_username")
    password := conf.GetString("jira_password")
    query := conf.GetString("jira_query")
    browserCommand := conf.GetString("browser_command")
    configColumns = conf.GetStringSlice("board_list")

    if containsEmpty(instanceURL, username, password, query, browserCommand) {
        fmt.Println("Sorry, couldn't find all required config variables.")
        printConfigHelp()
    }

    return configItem{
        instanceURL:    instanceURL,
        username:       username,
        password:       password,
        query:          query,
        browserCommand: browserCommand,
    }
}

func printConfigHelp() {
    fmt.Println(`
Here is an example config, the file should be placed under ~/.jb/jb.yaml or /etc/jb/jb.yaml :

jira_instance: "https://my.jira.instance.address"
jira_username: "my.username"
jira_password: "my.password"
board_list: ["Open", "In Progress", "On Hold", "Blocked External", "In Review"] # Or whichever statuses you want to display
browser_command: "/usr/bin/xdg-open" # Or any other specific browser path
jira_query: "project = TECH AND assignee = my.username AND status not in (Resolved, Closed, Rejected)" # Or any valid JQL
    `)
    os.Exit(0)
}

func updateStatus(g *gocui.Gui, msg string) {

    statusView, err := g.View("statusLine")
    if err != nil {
        // Probably the view didn't get created in parallel
        // We will skip, we are not blocking anything
        return
    }
    statusView.Clear()
    if msg == "" {
        return
    }
    fmt.Fprint(statusView, msg)
}

func cursorDown(g *gocui.Gui, v *gocui.View) error {
    if v != nil {
        cx, cy := v.Cursor()
        if err := v.SetCursor(cx, cy+1); err != nil {
            log.Warn("Problem setting the cursor down")
        }
    }
    return nil
}

func cursorUp(g *gocui.Gui, v *gocui.View) error {
    if v != nil {
        // ox, oy := v.Origin()
        _, oy := v.Origin()
        cx, cy := v.Cursor()
        if err := v.SetCursor(cx, cy-1); err != nil && oy > 0 {
            log.Warn("Problem setting the cursor up")
        }
    }
    return nil
}

func containsEmpty(ss ...string) bool {
    for _, s := range ss {
        if s == "" {
            return true
        }
    }
    return false
}

func main() {

    var logLevel = flag.String("loglevel", "info", "Accepted values are: info, debug, warn, fatal")
    var configHelp = flag.Bool("confighelp", false, "Prints the configuration help")

    flag.Usage = func() {
        fmt.Println()
        fmt.Println("jb is JIRA kanban board UI for terminal")
        fmt.Println()
        fmt.Println("Accepted parameters:")
        fmt.Println()
        flag.PrintDefaults()
    }

    flag.Parse()

    if *configHelp {
        printConfigHelp()
    }

    switch *logLevel {
        case "info", "debug", "warn", "fatal":
    default:
        fmt.Println("Could not understand requested loglevel")
        os.Exit(1)
    }
    file, err := os.OpenFile("/tmp/jb.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err == nil {
        requestedLevel, _ := logrus.ParseLevel(*logLevel)
        log.SetLevel(requestedLevel)
        log.Out = file
    } else {
        log.Info("Failed to log to file, using default stderr")
    }

    g, err := gocui.NewGui(gocui.OutputNormal)
    if err != nil {
        log.Panicln(err)
    }
    defer g.Close()

    g.Mouse = false
    g.Highlight = true
    g.Cursor = true
    g.InputEsc=true
    g.SelFgColor = gocui.ColorBlue
    g.SetManagerFunc(drawBoard)

    if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
        log.Panicln(err)
    }
    if err := g.SetKeybinding("menu", gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
        log.Panicln(err)
    }
    if err := g.SetKeybinding("menu", gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
        log.Panicln(err)
    }
    if err := g.SetKeybinding("menu", gocui.KeyEnter, gocui.ModNone, getMenuSelection); err != nil {
        log.Panicln(err)
    }
    if err := g.SetKeybinding("", gocui.KeyF5, gocui.ModNone, refreshBoard); err != nil {
        log.Panicln(err)
    }

    g.Update(func(g *gocui.Gui) error {
        return refreshBoard(g, nil)
    })

    if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
        log.Panicln(err)
    }

}
