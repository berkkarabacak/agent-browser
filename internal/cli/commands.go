package cli

import (
	"errors"
	"fmt"
	"strconv"

	"abrowser/internal/app"
	"abrowser/internal/extensions"
	"abrowser/internal/profile"
)

func (c *CLI) initCmd(args []string) int {
	if len(args) > 0 && isHelpToken(args[0]) {
		fmt.Fprint(c.IO.Out, "Usage: abrowser init\n\nCreates the data root, config.json, groups.json, and registry.json.\n")
		return 0
	}
	if len(args) > 0 {
		return c.usage("Usage: abrowser init\n")
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	if err := a.Init(); err != nil {
		return c.fail(err)
	}
	view, err := a.ShowConfig()
	if err != nil {
		return c.fail(err)
	}
	fmt.Fprintf(c.IO.Out, "Initialized %s\n", view.Root)
	if view.BrowserPath == "" {
		fmt.Fprintln(c.IO.Out, "Set the browser next:")
		fmt.Fprintln(c.IO.Out, "  abrowser config set browser C:\\path\\to\\chrome.exe")
	}
	return 0
}

func (c *CLI) configCmd(args []string) int {
	if len(args) == 0 || isHelpToken(args[0]) {
		return c.help(configHelp)
	}
	switch args[0] {
	case "show":
		return c.configShow(args[1:])
	case "set":
		return c.configSet(args[1:])
	default:
		fmt.Fprintf(c.IO.Err, "error: unknown config command %q\n\n", args[0])
		return c.usage(configHelp)
	}
}

func (c *CLI) configShow(args []string) int {
	fs := c.newFlags("config show", configHelp)
	asJSON := fs.Bool("json", c.JSON, "print JSON")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	if fs.NArg() != 0 {
		return c.usage(configHelp)
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	view, err := a.ShowConfig()
	if err != nil {
		return c.fail(err)
	}
	return c.printConfig(view, *asJSON)
}

func (c *CLI) configSet(args []string) int {
	if len(args) == 0 || isHelpToken(args[0]) {
		return c.help(configHelp)
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	key := args[0]
	rest := args[1:]
	var view app.ConfigView
	switch key {
	case "browser":
		if len(rest) != 1 {
			fmt.Fprintln(c.IO.Err, "error: browser path must be a single argument; quote paths that contain spaces")
			return 2
		}
		view, err = a.SetBrowser(rest[0])
	case "max-concurrent":
		if len(rest) != 1 {
			fmt.Fprintln(c.IO.Err, "error: max-concurrent requires one integer")
			return 2
		}
		n, nerr := strconv.Atoi(rest[0])
		if nerr != nil {
			return c.fail(fmt.Errorf("max-concurrent: %w", nerr))
		}
		view, err = a.SetMaxConcurrent(n)
	case "auto-close-lru":
		if len(rest) != 1 {
			fmt.Fprintln(c.IO.Err, "error: auto-close-lru requires true or false")
			return 2
		}
		v, berr := parseBool(rest[0])
		if berr != nil {
			return c.fail(berr)
		}
		view, err = a.SetAutoCloseLRU(v)
	case "extra-args":
		view, err = a.SetExtraArgs(rest)
	default:
		fmt.Fprintf(c.IO.Err, "error: unknown config key %q\n\n", key)
		return c.usage(configHelp)
	}
	if err != nil {
		return c.fail(err)
	}
	code := c.printConfig(view, c.JSON)
	if key == "browser" && !view.BrowserExists {
		fmt.Fprintf(c.IO.Err, "warning: %s does not exist yet\n", view.BrowserExpanded)
	}
	return code
}

func (c *CLI) extCmd(args []string) int {
	if len(args) == 0 || isHelpToken(args[0]) {
		return c.help(extHelp)
	}
	switch args[0] {
	case "list":
		return c.extList(args[1:])
	case "add":
		return c.extAdd(args[1:])
	case "remove":
		return c.extRemove(args[1:])
	default:
		fmt.Fprintf(c.IO.Err, "error: unknown ext command %q\n\n", args[0])
		return c.usage(extHelp)
	}
}

func (c *CLI) extList(args []string) int {
	fs := c.newFlags("ext list", extHelp)
	asJSON := fs.Bool("json", c.JSON, "print JSON")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	list, err := a.Extensions()
	if err != nil {
		return c.fail(err)
	}
	return c.printExtensions(list, *asJSON)
}

func (c *CLI) extAdd(args []string) int {
	fs := c.newFlags("ext add", extHelp)
	from := fs.String("from", "", "unpacked extension directory")
	crx := fs.String("crx", "", "path to a .crx file")
	version := fs.String("version", "", "manifest version; must match manifest.json when set")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		return c.usage(extHelp)
	}
	if (*from == "") == (*crx == "") {
		fmt.Fprintln(c.IO.Err, "error: pass exactly one of --from or --crx")
		return 2
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	name := fs.Arg(0)
	var ext extensions.Extension
	if *from != "" {
		ext, err = a.AddExtensionFromDir(name, *version, *from)
	} else {
		ext, err = a.AddExtensionFromCRX(name, *version, *crx)
	}
	if err != nil {
		return c.fail(err)
	}
	if c.JSON {
		return c.printJSON(ext)
	}
	fmt.Fprintf(c.IO.Out, "Added %s\n  %s\n", ext.Ref(), ext.Path)
	return 0
}

func (c *CLI) extRemove(args []string) int {
	fs := c.newFlags("ext remove", extHelp)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		return c.usage(extHelp)
	}
	token := fs.Arg(0)
	if !*yes && c.confirmYes(fmt.Sprintf("Remove extension %s from the store? [y/N]: ", token)) {
		*yes = true
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	ext, err := a.RemoveExtension(token, *yes)
	if err != nil {
		return c.fail(err)
	}
	if c.JSON {
		return c.printJSON(ext)
	}
	fmt.Fprintf(c.IO.Out, "Removed %s\n", ext.Ref())
	return 0
}

func (c *CLI) groupCmd(args []string) int {
	if len(args) == 0 || isHelpToken(args[0]) {
		return c.help(groupHelp)
	}
	switch args[0] {
	case "list":
		return c.groupList(args[1:])
	case "create":
		return c.groupCreate(args[1:])
	case "add":
		return c.groupAdd(args[1:])
	case "remove":
		return c.groupRemove(args[1:])
	default:
		fmt.Fprintf(c.IO.Err, "error: unknown group command %q\n\n", args[0])
		return c.usage(groupHelp)
	}
}

func (c *CLI) groupList(args []string) int {
	fs := c.newFlags("group list", groupHelp)
	asJSON := fs.Bool("json", c.JSON, "print JSON")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	g, err := a.Groups()
	if err != nil {
		return c.fail(err)
	}
	return c.printGroups(g, *asJSON)
}

func (c *CLI) groupCreate(args []string) int {
	if len(args) == 1 && isHelpToken(args[0]) {
		return c.help(groupHelp)
	}
	if len(args) != 1 {
		return c.usage(groupHelp)
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	if err := a.CreateGroup(args[0]); err != nil {
		return c.fail(err)
	}
	fmt.Fprintf(c.IO.Out, "Created group %s\n", args[0])
	return 0
}

func (c *CLI) groupAdd(args []string) int {
	if len(args) > 0 && isHelpToken(args[0]) {
		return c.help(groupHelp)
	}
	if len(args) != 2 {
		return c.usage(groupHelp)
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	change, err := a.GroupAdd(args[0], args[1])
	if err != nil {
		return c.fail(err)
	}
	if c.JSON {
		return c.printJSON(change)
	}
	if change.Already {
		fmt.Fprintf(c.IO.Out, "Group %s already includes %s\n", change.Group, change.Extension)
		return 0
	}
	fmt.Fprintf(c.IO.Out, "Added %s to %s\n", change.Extension, change.Group)
	return 0
}

func (c *CLI) groupRemove(args []string) int {
	if len(args) > 0 && isHelpToken(args[0]) {
		return c.help(groupHelp)
	}
	if len(args) != 2 {
		return c.usage(groupHelp)
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	if err := a.GroupRemove(args[0], args[1]); err != nil {
		return c.fail(err)
	}
	fmt.Fprintf(c.IO.Out, "Removed %s from %s\n", args[1], args[0])
	return 0
}

func (c *CLI) templateCmd(args []string) int {
	if len(args) == 0 || isHelpToken(args[0]) {
		return c.help(templateHelp)
	}
	switch args[0] {
	case "create":
		return c.templateCreate(args[1:])
	case "launch":
		return c.templateLaunch(args[1:])
	case "close":
		return c.templateClose(args[1:])
	case "freeze":
		return c.templateFreeze(args[1:])
	case "unfreeze":
		return c.templateUnfreeze(args[1:])
	case "list":
		return c.templateList(args[1:])
	case "delete":
		return c.templateDelete(args[1:])
	case "set":
		return c.templateSet(args[1:])
	case "group":
		return c.assignmentGroup("template", args[1:])
	case "ext":
		return c.assignmentExt("template", args[1:])
	default:
		fmt.Fprintf(c.IO.Err, "error: unknown template command %q\n\n", args[0])
		return c.usage(templateHelp)
	}
}

func (c *CLI) templateCreate(args []string) int {
	fs := c.newFlags("template create", templateHelp)
	tz := fs.String("timezone", "", "IANA time zone; host zone if omitted")
	locale := fs.String("locale", "", "locale passed to Chromium (default en-US)")
	notes := fs.String("notes", "", "notes")
	var groups stringList
	fs.Var(&groups, "group", "extension group to assign (repeatable)")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		return c.usage(templateHelp)
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	created, err := a.CreateTemplate(app.TemplateCreate{
		Name:     fs.Arg(0),
		Timezone: *tz,
		Locale:   *locale,
		Notes:    *notes,
		Groups:   []string(groups),
	})
	if err != nil {
		return c.fail(err)
	}
	if c.JSON {
		return c.printJSON(created)
	}
	fmt.Fprintf(c.IO.Out, "Created template %s\n", created.Name)
	fmt.Fprintf(c.IO.Out, "  locale:   %s\n", created.Locale)
	fmt.Fprintf(c.IO.Out, "  timezone: %s\n", zoneOrHost(created.Timezone))
	fmt.Fprintf(c.IO.Out, "  groups:   %s\n", joinOrDash(created.Groups))
	fmt.Fprintf(c.IO.Out, "Launch it to configure, then freeze:\n  abrowser template launch %s\n  abrowser template freeze %s\n", created.Name, created.Name)
	return 0
}

func (c *CLI) templateLaunch(args []string) int {
	fs := c.newFlags("template launch", templateHelp)
	dry := fs.Bool("dry-run", false, "print the Chromium command without starting it")
	asJSON := fs.Bool("json", c.JSON, "print JSON")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		return c.usage(templateHelp)
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	res, err := a.LaunchTemplate(fs.Arg(0), *dry)
	if err != nil {
		return c.fail(err)
	}
	return c.printLaunch(res, *asJSON)
}

func (c *CLI) templateClose(args []string) int {
	if len(args) == 1 && isHelpToken(args[0]) {
		return c.help(templateHelp)
	}
	if len(args) != 1 {
		return c.usage(templateHelp)
	}
	return c.stop("template", args[0])
}

func (c *CLI) templateFreeze(args []string) int {
	if len(args) == 1 && isHelpToken(args[0]) {
		return c.help(templateHelp)
	}
	if len(args) != 1 {
		return c.usage(templateHelp)
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	res, err := a.FreezeTemplate(args[0])
	if err != nil {
		return c.fail(err)
	}
	if c.JSON {
		return c.printJSON(res)
	}
	fmt.Fprintf(c.IO.Out, "Froze template %s\n", res.Name)
	fmt.Fprintf(c.IO.Out, "  user data kept: %d files, %s (caches not included)\n", res.Files, profile.FormatBytes(res.Bytes))
	return 0
}

func (c *CLI) templateUnfreeze(args []string) int {
	if len(args) == 1 && isHelpToken(args[0]) {
		return c.help(templateHelp)
	}
	if len(args) != 1 {
		return c.usage(templateHelp)
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	if err := a.UnfreezeTemplate(args[0]); err != nil {
		return c.fail(err)
	}
	fmt.Fprintf(c.IO.Out, "Unfroze template %s. Launch it to edit, then freeze it again before cloning.\n", args[0])
	return 0
}

func (c *CLI) templateList(args []string) int {
	fs := c.newFlags("template list", templateHelp)
	asJSON := fs.Bool("json", c.JSON, "print JSON")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	inv, err := a.List()
	if err != nil {
		return c.fail(err)
	}
	return c.printInventory(inv, *asJSON, "templates")
}

func (c *CLI) templateDelete(args []string) int {
	fs := c.newFlags("template delete", templateHelp)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		return c.usage(templateHelp)
	}
	name := fs.Arg(0)
	if !*yes && c.confirmExact(name, fmt.Sprintf("Type %q to delete template %s: ", name, name)) {
		*yes = true
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	if err := a.DeleteTemplate(name, *yes); err != nil {
		return c.fail(err)
	}
	fmt.Fprintf(c.IO.Out, "Deleted template %s\n", name)
	return 0
}

func (c *CLI) templateSet(args []string) int {
	fs := c.newFlags("template set", templateHelp)
	tz := fs.String("timezone", "", "IANA time zone")
	locale := fs.String("locale", "", "browser locale")
	notes := fs.String("notes", "", "notes")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		return c.usage(templateHelp)
	}
	var u app.FieldUpdates
	if flagSeen(fs, "timezone") {
		u.Timezone = strPtr(*tz)
	}
	if flagSeen(fs, "locale") {
		u.Locale = strPtr(*locale)
	}
	if flagSeen(fs, "notes") {
		u.Notes = strPtr(*notes)
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	t, err := a.UpdateTemplate(fs.Arg(0), u)
	if err != nil {
		return c.fail(err)
	}
	if c.JSON {
		return c.printJSON(t)
	}
	fmt.Fprintf(c.IO.Out, "Updated template %s\n", t.Name)
	return 0
}

func (c *CLI) cloneCmd(args []string) int {
	fs := c.newFlags("clone", cloneHelp)
	as := fs.String("as", "", "new profile id")
	email := fs.String("email", "", "account email")
	tz := fs.String("timezone", "", "IANA time zone")
	locale := fs.String("locale", "", "browser locale")
	notes := fs.String("notes", "", "notes")
	var groups, add, remove stringList
	fs.Var(&groups, "group", "extension group (repeatable; replaces template groups)")
	fs.Var(&add, "add-ext", "+ext override (repeatable; replaces template adds)")
	fs.Var(&remove, "remove-ext", "-ext override (repeatable; replaces template removals)")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 || *as == "" || *email == "" {
		return c.usage(cloneHelp)
	}
	req := app.CloneRequest{
		Template:  fs.Arg(0),
		ID:        *as,
		Email:     *email,
		Groups:    []string(groups),
		GroupsSet: flagSeen(fs, "group"),
		Add:       []string(add),
		AddSet:    flagSeen(fs, "add-ext"),
		Remove:    []string(remove),
		RemoveSet: flagSeen(fs, "remove-ext"),
	}
	if flagSeen(fs, "timezone") {
		req.Timezone = strPtr(*tz)
	}
	if flagSeen(fs, "locale") {
		req.Locale = strPtr(*locale)
	}
	if flagSeen(fs, "notes") {
		req.Notes = strPtr(*notes)
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	res, err := a.Clone(req)
	if err != nil {
		return c.fail(err)
	}
	if c.JSON {
		return c.printJSON(res)
	}
	fmt.Fprintf(c.IO.Out, "Cloned %s -> %s (%s)\n", res.Profile.Template, res.Profile.ID, res.Profile.Email)
	fmt.Fprintf(c.IO.Out, "  files:    %d (%s of user data, caches skipped)\n", res.Files, profile.FormatBytes(res.Bytes))
	fmt.Fprintf(c.IO.Out, "  groups:   %s\n", joinOrDash(res.Profile.Groups))
	fmt.Fprintf(c.IO.Out, "  timezone: %s\n", zoneOrHost(res.Profile.Timezone))
	fmt.Fprintf(c.IO.Out, "  locale:   %s\n", emptyDash(res.Profile.Locale))
	return 0
}

func (c *CLI) listCmd(args []string) int {
	fs := c.newFlags("list", launchHelp)
	asJSON := fs.Bool("json", c.JSON, "print JSON")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	inv, err := a.List()
	if err != nil {
		return c.fail(err)
	}
	return c.printInventory(inv, *asJSON, "all")
}

func (c *CLI) launchCmd(args []string) int {
	fs := c.newFlags("launch", launchHelp)
	dry := fs.Bool("dry-run", false, "print the Chromium command without starting it")
	asJSON := fs.Bool("json", c.JSON, "print JSON")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		return c.usage(launchHelp)
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	res, err := a.LaunchProfile(fs.Arg(0), *dry)
	if err != nil {
		return c.fail(err)
	}
	return c.printLaunch(res, *asJSON)
}

func (c *CLI) closeCmd(args []string) int {
	if len(args) == 1 && isHelpToken(args[0]) {
		return c.help(launchHelp)
	}
	if len(args) != 1 {
		return c.usage(launchHelp)
	}
	return c.stop("profile", args[0])
}

func (c *CLI) stop(kind, id string) int {
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	var call error
	if kind == "template" {
		call = a.CloseTemplate(id)
	} else {
		call = a.CloseProfile(id)
	}
	if call != nil {
		var nr *app.NotRunningError
		if errors.As(call, &nr) {
			fmt.Fprintln(c.IO.Out, call.Error())
			return 0
		}
		return c.fail(call)
	}
	fmt.Fprintf(c.IO.Out, "Closed %s %s\n", kind, id)
	return 0
}

func (c *CLI) deleteCmd(args []string) int {
	fs := c.newFlags("delete", launchHelp)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		return c.usage(launchHelp)
	}
	id := fs.Arg(0)
	if !*yes && c.confirmExact(id, fmt.Sprintf("Type %q to delete profile %s and its user data: ", id, id)) {
		*yes = true
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	if err := a.DeleteProfile(id, *yes); err != nil {
		return c.fail(err)
	}
	fmt.Fprintf(c.IO.Out, "Deleted profile %s\n", id)
	return 0
}

func (c *CLI) statusCmd(args []string) int {
	fs := c.newFlags("status", launchHelp)
	asJSON := fs.Bool("json", c.JSON, "print JSON")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	st, err := a.Status()
	if err != nil {
		return c.fail(err)
	}
	return c.printStatus(st, *asJSON)
}

func (c *CLI) profileCmd(args []string) int {
	if len(args) == 0 || isHelpToken(args[0]) {
		return c.help(profileHelp)
	}
	switch args[0] {
	case "show":
		return c.profileShow(args[1:])
	case "set":
		return c.profileSet(args[1:])
	case "group":
		return c.assignmentGroup("profile", args[1:])
	case "ext":
		return c.assignmentExt("profile", args[1:])
	default:
		fmt.Fprintf(c.IO.Err, "error: unknown profile command %q\n\n", args[0])
		return c.usage(profileHelp)
	}
}

func (c *CLI) profileShow(args []string) int {
	fs := c.newFlags("profile show", profileHelp)
	asJSON := fs.Bool("json", c.JSON, "print JSON")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		return c.usage(profileHelp)
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	view, err := a.ShowProfile(fs.Arg(0))
	if err != nil {
		return c.fail(err)
	}
	return c.printProfile(view, *asJSON)
}

func (c *CLI) profileSet(args []string) int {
	fs := c.newFlags("profile set", profileHelp)
	email := fs.String("email", "", "account email")
	tz := fs.String("timezone", "", "IANA time zone")
	locale := fs.String("locale", "", "browser locale")
	notes := fs.String("notes", "", "notes")
	if code, ok := c.parse(fs, args); !ok {
		return code
	}
	if fs.NArg() != 1 {
		return c.usage(profileHelp)
	}
	var u app.FieldUpdates
	if flagSeen(fs, "email") {
		u.Email = strPtr(*email)
	}
	if flagSeen(fs, "timezone") {
		u.Timezone = strPtr(*tz)
	}
	if flagSeen(fs, "locale") {
		u.Locale = strPtr(*locale)
	}
	if flagSeen(fs, "notes") {
		u.Notes = strPtr(*notes)
	}
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	p, err := a.UpdateProfile(fs.Arg(0), u)
	if err != nil {
		return c.fail(err)
	}
	if c.JSON {
		return c.printJSON(p)
	}
	fmt.Fprintf(c.IO.Out, "Updated profile %s\n", p.ID)
	return 0
}

func (c *CLI) assignmentGroup(kind string, args []string) int {
	helpText := profileHelp
	if kind == "template" {
		helpText = templateHelp
	}
	if len(args) == 0 || isHelpToken(args[0]) {
		return c.help(helpText)
	}
	if len(args) != 3 || (args[0] != "add" && args[0] != "remove") {
		return c.usage(helpText)
	}
	op, id, group := args[0], args[1], args[2]
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	if kind == "template" {
		t, err := a.TemplateGroup(id, op, group)
		if err != nil {
			return c.fail(err)
		}
		if c.JSON {
			return c.printJSON(t)
		}
		fmt.Fprintf(c.IO.Out, "Template %s groups: %s\n", t.Name, joinOrDash(t.Groups))
		return 0
	}
	p, err := a.ProfileGroup(id, op, group)
	if err != nil {
		return c.fail(err)
	}
	if c.JSON {
		return c.printJSON(p)
	}
	fmt.Fprintf(c.IO.Out, "Profile %s groups: %s\n", p.ID, joinOrDash(p.Groups))
	return 0
}

func (c *CLI) assignmentExt(kind string, args []string) int {
	helpText := profileHelp
	if kind == "template" {
		helpText = templateHelp
	}
	if len(args) == 0 || isHelpToken(args[0]) {
		return c.help(helpText)
	}
	if len(args) != 3 || (args[0] != "add" && args[0] != "remove" && args[0] != "reset") {
		return c.usage(helpText)
	}
	op, id, ref := args[0], args[1], args[2]
	a, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	if kind == "template" {
		t, err := a.TemplateExt(id, op, ref)
		if err != nil {
			return c.fail(err)
		}
		if c.JSON {
			return c.printJSON(t)
		}
		fmt.Fprintf(c.IO.Out, "Template %s overrides: add %s; remove %s\n", t.Name, joinOrDash(t.AddExtensions), joinOrDash(t.RemoveExtensions))
		return 0
	}
	p, err := a.ProfileExt(id, op, ref)
	if err != nil {
		return c.fail(err)
	}
	if c.JSON {
		return c.printJSON(p)
	}
	fmt.Fprintf(c.IO.Out, "Profile %s overrides: add %s; remove %s\n", p.ID, joinOrDash(p.AddExtensions), joinOrDash(p.RemoveExtensions))
	return 0
}
