package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	jira "github.com/andygrunwald/go-jira"
	"github.com/jroimartin/gocui"
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
	logicalMx     = []column{}
	active        = &activeBox{}
	jiraClient    = &jira.Client{}
	configColumns = []string{}
	moveCounter   = 0
	infoText      = "Navigation: Arrow keys  |  Actions Menu: Spacebar  |  Exit: Ctrl-C"
)

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
	for i := range logicalMx {
		if len(logicalMx[i].members) > 0 {
			setCurrentViewOnTop(g, logicalMx[i].members[0].view.Title)
			active.columnname = logicalMx[i].view.Title
			active.issuetitle = logicalMx[i].members[0].view.Title
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

func registerIssue(box issueBox) {

	for i := range logicalMx {
		if logicalMx[i].view.Title == box.issue.Fields.Status.Name {
			logicalMx[i].members = append(logicalMx[i].members, box)
		}
	}

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
		if err := g.SetKeybinding("msgBox", gocui.KeyCtrlD, gocui.ModNone, destroyView); err != nil {
			log.Panicln(err)
		}
		if err := g.SetKeybinding("msgBox", gocui.KeyCtrlS, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			updateStatus(g, "Sorry, not implemented yet!")
			destroyView(g, v)
			return nil
		}); err != nil {
			log.Panicln(err)
		}
		updateStatus(g, "Send: Ctrl-S (not implemented yet)  |  Close: Ctrl-D")
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
			for i := range logicalMx {
				if logicalMx[i].view.Title == active.columnname {
					issue := logicalMx[i].members[active.indexno].issue
					lineSlice := strings.SplitN(
						issue.Fields.Description,
						"\r\n",
						-1,
					)
					fmt.Fprintln(v, "Reporter: "+issue.Fields.Reporter.DisplayName)
					// fmt.Fprintln(v, "Assignee: "+issue.Fields.Assignee.DisplayName)
					// fmt.Fprintln(v, "Labels: "+strings.Join(issue.Fields.Labels, ","))
					// fmt.Fprintln(v, "Priority: "+issue.Fields.Priority.Name)
					// TODO: Find a way to handle exceptions here,
					// TODO: Sometimes fields (like .Assignee) is nil
					fmt.Fprint(v, "Description:\n\n")
					for i := range lineSlice {
						fmt.Fprintln(v, lineSlice[i])
					}
					// fmt.Fprint(v, "\nComments:\n")
					// comments := issue.Fields.Comments.Comments
					// for i := range comments {
					// 	fmt.Fprintln(
					// 		v,
					// 		comments[i].Author.DisplayName+" : "+comments[i].Body,
					// 	)
					// }
					break
				}
			}
			setCurrentViewOnTop(g, "previewBox")
		}
		if err := g.SetKeybinding("previewBox", gocui.KeyCtrlD, gocui.ModNone, destroyView); err != nil {
			log.Panicln(err)
		}
		updateStatus(g, "Close: Ctrl-D")
	case "Open in browser":
		conf := readConfig()
		issueURL := ""
		for k := range logicalMx {
			if logicalMx[k].view.Title == active.columnname {
				issueURL = conf.instanceURL + "/browse/" + logicalMx[k].members[active.indexno].issue.Key
				break
			}
		}
		if issueURL != "" {
			cmd := exec.Command(conf.browserCommand, issueURL)
			err := cmd.Start()
			if err != nil {
				log.Fatal(err)
			}
			menuView, _ := g.View("menu")
			destroyView(g, menuView)
		} else {
			updateStatus(g, "Couldn't find the issue URL, sorry.")
		}
	case "":
		// WHY U SELECT NOTHING?
		// I'LL CLOSE THE MENU!!
		menuView, _ := g.View("menu")
		destroyView(g, menuView)
	default:
		for i := range logicalMx {
			if logicalMx[i].view.Title == active.columnname {
				jiraAction(g, &logicalMx[i].members[active.indexno].issue, line)
				break
			}
		}
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

	_, maxY := g.Size()
	activeIssue := jira.Issue{}
	for i := range logicalMx {
		if logicalMx[i].view.Title == active.columnname {
			xzero, _, _, yone, _ := g.ViewPosition(logicalMx[i].members[active.indexno].view.Title)
			if yone > maxY-14 {
				// Menu will not fit, so let's lift it up a bit
				menuCoord = [4]int{xzero, yone - 14, xzero + 32, yone}
			} else {
				menuCoord = [4]int{xzero, yone, xzero + 32, yone + 14}
			}
			activeIssue = logicalMx[i].members[active.indexno].issue
		}
	}

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
		for i := range actionMap {
			fmt.Fprintln(v, "Send to -> "+i)
		}
		fmt.Fprintln(v, "Open in browser")
		fmt.Fprintln(v, "Preview issue")
		fmt.Fprintln(v, "Comment on issue")
	}
	if err := g.SetKeybinding("menu", gocui.KeySpace, gocui.ModNone, destroyView); err != nil {
		log.Panicln(err)
	}

	updateStatus(g, "Run action: Enter  |  Close menu: Spacebar")
	setCurrentViewOnTop(g, "menu")

	return nil
}

func refreshBoard(g *gocui.Gui, v *gocui.View) error {

	updateStatus(g, "Refreshing board...")

	for i := range logicalMx {
		for m := range logicalMx[i].members {
			g.DeleteKeybindings(logicalMx[i].members[m].view.Title)
			g.DeleteView(logicalMx[i].members[m].view.Title)
		}
	}
	for i := range logicalMx {
		logicalMx[i].members = logicalMx[i].members[:0]
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
		//TODO error catch
		issueCoord = [4]int{xzero, yone + 1, xone, yone + 6}
	}
	return issueCoord, nil

}

func createIssue(g *gocui.Gui, issue jira.Issue) error {

	correctColumn := column{}
	undefinedColumn := true

	for i := range logicalMx {
		if logicalMx[i].view.Title == issue.Fields.Status.Name {
			correctColumn = logicalMx[i]
			undefinedColumn = false
			break
		}
	}

	if undefinedColumn {
		//The issue's status is not in our column list,
		updateStatus(g, "Issue with undefined column found and skipped.")
		return nil
	}

	validCoords, _ := giveNextIssueCoord(g, correctColumn)
	// TODO error catch

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
		if err := g.SetKeybinding(issue.Key, gocui.KeyArrowDown, gocui.ModNone, downView); err != nil {
			log.Panicln(err)
		}
		if err := g.SetKeybinding(issue.Key, gocui.KeyArrowUp, gocui.ModNone, upView); err != nil {
			log.Panicln(err)
		}
		if err := g.SetKeybinding(issue.Key, gocui.KeyArrowRight, gocui.ModNone, rightView); err != nil {
			log.Panicln(err)
		}
		if err := g.SetKeybinding(issue.Key, gocui.KeyArrowLeft, gocui.ModNone, leftView); err != nil {
			log.Panicln(err)
		}
		if err := g.SetKeybinding(issue.Key, gocui.KeySpace, gocui.ModNone, openMenu); err != nil {
			log.Panicln(err)
		}
	}

	return nil
}

func upView(g *gocui.Gui, v *gocui.View) error {

	newView := &gocui.View{}

	for i := range logicalMx {
		if logicalMx[i].view.Title == active.columnname {
			if active.indexno == 0 {
				// We're already on top, go back to bottom
				active.indexno = len(logicalMx[i].members) - 1
				active.issuetitle = logicalMx[i].members[active.indexno].view.Title
				newView = logicalMx[i].members[active.indexno].view
				moveIssues(g, 0, true)
				break
			}
			active.indexno = active.indexno - 1
			active.issuetitle = logicalMx[i].members[active.indexno].view.Title
			newView = logicalMx[i].members[active.indexno].view
			_, y0, _, _, _ := g.ViewPosition(newView.Title)
			if y0 < 0 {
				moveIssues(g, 6, false)
			}
			break
		}
	}

	if _, err := setCurrentViewOnTop(g, newView.Title); err != nil {
		return err
	}

	updateStatus(g, "")

	return nil
}

func downView(g *gocui.Gui, v *gocui.View) error {

	newView := &gocui.View{}

	for i := range logicalMx {
		if logicalMx[i].view.Title == active.columnname {
			if len(logicalMx[i].members) == active.indexno+1 {
				// We're already on bottom, let's check one more thing
				if len(logicalMx[i].members) == 1 {
					// Dude, you only have one box here
					return nil
				}
				// Ok we're going to top
				active.indexno = 0
				active.issuetitle = logicalMx[i].members[0].view.Title
				newView = logicalMx[i].members[0].view
				moveIssues(g, 0, true)
				break
			}
			active.indexno = active.indexno + 1
			active.issuetitle = logicalMx[i].members[active.indexno].view.Title
			newView = logicalMx[i].members[active.indexno].view
			_, maxY := g.Size()
			_, _, _, y1, _ := g.ViewPosition(newView.Title)
			if y1 > maxY-3 {
				moveIssues(g, -6, false)
			}
			break
		}
	}

	if _, err := setCurrentViewOnTop(g, newView.Title); err != nil {
		return err
	}

	updateStatus(g, "")

	return nil
}

func moveIssues(g *gocui.Gui, dy int, reset bool) error {
	if reset {
		// This is a reset call
		// Let's see if we're going to bottom or top
		goingToTop := false
		if moveCounter != 0 {
			// We're in moved position, request must be top
			goingToTop = true
		}
		if dy == 1000 {
			// This is right-left arrow, we will reset to top
			goingToTop = true
		}
		if goingToTop {
			// We are going to top
			for i := range logicalMx {
				if logicalMx[i].view.Title == active.columnname {
					for vi := range logicalMx[i].members {
						curView := logicalMx[i].members[vi].view.Title
						c1, c2, c3, c4, _ := g.ViewPosition(curView)
						if moveCounter > 0 {
							if _, err := g.SetView(curView, c1, c2+6*moveCounter, c3, c4+6*moveCounter); err != nil {
								return err
							}
						} else if moveCounter < 0 {
							if _, err := g.SetView(curView, c1, c2-6*moveCounter, c3, c4-6*moveCounter); err != nil {
								return err
							}
						}
					}
				}
				moveCounter = 0
			}
		} else {
			// We are going to bottom
			for i := range logicalMx {
				if logicalMx[i].view.Title == active.columnname {
					issueCount := len(logicalMx[i].members)
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

	for i := range logicalMx {
		if logicalMx[i].view.Title == active.columnname {
			for vi := range logicalMx[i].members {
				curView := logicalMx[i].members[vi].view.Title
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

func rightView(g *gocui.Gui, v *gocui.View) error {

	newView := &gocui.View{}

	nextColumnName := active.columnname

	columnFound := false
	for !columnFound {
		for i := range logicalMx {
			if logicalMx[i].view.Title == nextColumnName {
				if i == len(logicalMx)-1 {
					nextColumnName = logicalMx[0].view.Title
					if len(logicalMx[0].members) > 0 {
						columnFound = true
					}
					break
				}
				nextColumnName = logicalMx[i+1].view.Title
				if len(logicalMx[i+1].members) > 0 {
					columnFound = true
				}
				break
			}
		}
	}

	boxFound := false
	for !boxFound {
		for i := range logicalMx {
			if logicalMx[i].view.Title == nextColumnName {
				if len(logicalMx[i].members) == 0 {
					// Empty column, skip
					nextColumnName = logicalMx[(i+1)%(len(logicalMx)-1)].view.Title
					break
				}
				// Not empty, let's check the size
				// is big enough to jump right in
				if len(logicalMx[i].members) > active.indexno {
					active.issuetitle = logicalMx[i].members[active.indexno].view.Title
					newView = logicalMx[i].members[active.indexno].view
				} else {
					active.indexno = len(logicalMx[i].members) - 1
					active.issuetitle = logicalMx[i].members[active.indexno].view.Title
					newView = logicalMx[i].members[active.indexno].view
				}
				boxFound = true
				break
			}
		}
	}

	// Reset the old column's replacement since we're leaving it
	moveIssues(g, 1000, true)
	if _, err := setCurrentViewOnTop(g, newView.Title); err != nil {
		return err
	}

	updateStatus(g, "")
	active.columnname = nextColumnName
	return nil
}

func leftView(g *gocui.Gui, v *gocui.View) error {

	newView := &gocui.View{}

	prevColumnName := active.columnname

	columnFound := false
	for !columnFound {
		for i := range logicalMx {
			if logicalMx[i].view.Title == prevColumnName {
				if i == 0 {
					prevColumnName = logicalMx[len(logicalMx)-1].view.Title
					if len(logicalMx[len(logicalMx)-1].members) > 0 {
						columnFound = true
					}
					break
				}
				prevColumnName = logicalMx[i-1].view.Title
				if len(logicalMx[i-1].members) > 0 {
					columnFound = true
				}
				break
			}
		}
	}

	boxFound := false
	for !boxFound {
		for i := range logicalMx {
			if logicalMx[i].view.Title == prevColumnName {
				if len(logicalMx[i].members) == 0 {
					// Empty column, skip
					if i == 0 {
						prevColumnName = logicalMx[len(logicalMx)-1].view.Title
					} else {
						prevColumnName = logicalMx[i-1].view.Title
					}
					break
				}
				// Not empty, let's check the size
				// is big enough to jump right in
				if len(logicalMx[i].members) > active.indexno {
					active.issuetitle = logicalMx[i].members[active.indexno].view.Title
					newView = logicalMx[i].members[active.indexno].view
				} else {
					active.indexno = len(logicalMx[i].members) - 1
					active.issuetitle = logicalMx[i].members[active.indexno].view.Title
					newView = logicalMx[i].members[active.indexno].view
				}
				boxFound = true
				break
			}
		}
	}

	// Reset the old column's replacement since we're leaving it
	moveIssues(g, 1000, true)
	if _, err := setCurrentViewOnTop(g, newView.Title); err != nil {
		return err
	}

	updateStatus(g, "")
	active.columnname = prevColumnName
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

func drawBoard(g *gocui.Gui) error {
	maxX, realmaxY := g.Size()

	//Bottom area will be used for status line
	maxY := realmaxY - 1

	// This is needed (especially in parallel execution or refresh events),
	//since configColumns will be filled with reading configuration.
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
			logicalMx = append(logicalMx, newcol)

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
		fmt.Println(
			`
Sorry, couldn't find all required config variables.

Here is an example config, the file should be placed under ~/.jb/jb.yaml :

jira_instance: "https://my.jira.instance.address"
jira_username: "my.username"
jira_password: "my.password"
board_list: ["Open", "In Progress", "On Hold", "Blocked External", "In Review"] # Or whichever statuses you want to display
browser_command: "/usr/bin/xdg-open" # Or any other specific browser path
jira_query: "project = TECH AND assignee = my.username AND status not in (Resolved, Closed, Rejected)" # Or any valid JQL

`)
		os.Exit(0)
	}

	return configItem{
		instanceURL:    instanceURL,
		username:       username,
		password:       password,
		query:          query,
		browserCommand: browserCommand,
	}
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
			// ox, oy := v.Origin()
			// if err := v.SetOrigin(ox, oy+1); err != nil {
			// 	return err
			// }
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
			// if err := v.SetOrigin(ox, oy-1); err != nil {
			// 	return err
			// }
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

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	g.Mouse = false
	g.Highlight = true
	g.Cursor = true
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
