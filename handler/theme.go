package handler

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/PuerkitoBio/goquery"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"html"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

var (
	LOCATIONS = []ThemeLayoutLocationView{{
		Name: "homepage",
		Title: "Homepage",
		Path: path.Join("layouts", "index.html"),
	},{
		Name: "category",
		Title: "Category",
		Path: path.Join("layouts", "categories", "list.html"),
	},{
		Name: "products",
		Title: "Product",
		Path: path.Join("layouts", "products", "single.html"),
	},
	}
	//
	reRemoveRelative = regexp.MustCompile(`^\./`)
)

type ThemeShortView struct {
	Name string `json:",omitempty"`
	Size int64
	ModTime time.Time
}

type ThemesShortView []ThemeShortView

// @security BasicAuth
// GetThemes godoc
// @Summary Get themes
// @Accept json
// @Produce json
// @Success 200 {object} ThemesShortView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/themes [get]
// @Tags theme
func getThemesHandler(c *fiber.Ctx) error {
	var themes []ThemeShortView
	files, err := ioutil.ReadDir(path.Join(dir, "hugo", "themes"))
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	for _, f := range files {
		if f.IsDir() {
			f.ModTime()
			themes = append(themes, ThemeShortView{Name:f.Name(), Size: f.Size(), ModTime: f.ModTime()})
		}
	}
	c.Status(http.StatusOK)
	return c.JSON(themes)
}

type ThemeView struct {
	Name string `json:",omitempty"`
	Title string `json:",omitempty"`
	Layouts []ThemeLayoutLocationView
	Size int64
	ModTime time.Time
}

type ThemeLayoutLocationView struct {
	Name string
	Title string
	Path string
}

// @security BasicAuth
// GetThemes godoc
// @Summary Get themes
// @Accept json
// @Produce json
// @Success 200 {object} ThemeView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/themes/{name} [get]
// @Tags theme
func getThemeHandler(c *fiber.Ctx) error {
	var name string
	if name = c.Params("name"); name == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Name not set"})
	}

	var theme ThemeView

	theme.Name = name

	if fi, err := os.Stat(path.Join(dir, "hugo", "themes", name)); err == nil {
		theme.Size = fi.Size()
		theme.ModTime = fi.ModTime()
	} else {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}

	var conf struct {
		Name string `toml:"name"`
	}

	p := path.Join(dir, "hugo", "themes", name)

	if _, err := toml.DecodeFile(path.Join(p, "theme.toml"), &conf); err == nil {
		theme.Title = conf.Name
	} else {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}

	for _, location := range LOCATIONS {
		if _, err := os.Stat(path.Join(p, location.Path)); err == nil {
			theme.Layouts = append(theme.Layouts, location)
		}
	}

	c.Status(http.StatusOK)
	return c.JSON(theme)
}

type ThemeLayoutView struct {
	Path string
	Name string
	Locations []ThemeLayoutLocation
	Plugins []ThemeLayoutPluginView
	Size int64
	ModTime time.Time
}

type ThemeLayoutLocation struct {
	Name string
	Title string
	Plugins []ThemeLayoutPluginInstanceView
}

type ThemeLayoutPluginView struct {
	Name string
	Title string
	Description string
	Params []PluginParamView
	Size int64
	ModTime time.Time
}

type ThemeLayoutPluginInstanceView struct{
	Name string
	Title string
	Params []PluginParamView
}

type PluginParamView struct {
	Name string
	Description string `json:",omitempty"`
	Type string `json:",omitempty"`
	Value interface{} `json:",omitempty"`
	Values []string `json:",omitempty"`
}

// @security BasicAuth
// GetThemeLayout godoc
// @Summary Get theme layout
// @Accept json
// @Produce json
// @Success 200 {object} ThemeLayoutView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/themes/{name}/layouts/{any} [get]
// @Tags theme
func getThemeLayoutHandler(c *fiber.Ctx) error {
	var name string
	if name = c.Params("name"); name == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Name not set"})
	}

	p := strings.Replace(string(c.Request().URI().Path()), "/api/v1/themes/" + name + "/layouts", "", 1)
	p = strings.Replace(p, "../", "", -1)

	var layout ThemeLayoutView

	layout.Path = path.Dir(p)
	layout.Name = path.Base(p)
	layout.Plugins = []ThemeLayoutPluginView{}

	// Plugins
	var plugins []ThemeLayoutPluginView
	if p := path.Join(dir, "hugo", "themes", name, "layouts",  "partials", "plugins"); p != "" {
		files, err := ioutil.ReadDir(p)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		for _, f := range files {
			if f.IsDir() {
				var conf struct {
					Name string
					Description string
					Params []PluginParamView
				}
				var plugin ThemeLayoutPluginView
				if fi, err := os.Stat(path.Join(p, f.Name(), "index.json")); err == nil {
					plugin.Name = f.Name()
					plugin.Size = fi.Size()
					plugin.ModTime = fi.ModTime()
				}
				if bts, err := ioutil.ReadFile(path.Join(p, f.Name(), "index.json")); err == nil {
					if err = json.Unmarshal(bts, &conf); err == nil {
						plugin.Title = conf.Name
						plugin.Description = conf.Description
						plugin.Params = conf.Params
						plugins = append(plugins, plugin)
					}else{
						logger.Warningf("%+v", err)
					}
				}else{
					logger.Warningf("%+v", err)
				}
			}
		}
		layout.Plugins = plugins
	}

	filepath := path.Join(dir, "hugo", "themes", name, "layouts", p)
	if fi, err := os.Stat(filepath); err == nil {
		if bts, err := ioutil.ReadFile(filepath); err == nil {

			doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bts))
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			doc.Find("[data-place]").Each(func(i int, s *goquery.Selection) {
				var place string
				if v, exists := s.Attr("data-place"); exists {
					place = v
				}
				var title string
				if v, exists := s.Attr("data-title"); exists {
					title = v
				}
				var found bool
				for _, location := range layout.Locations {
					if location.Name == place {
						found = true
						break
					}
				}
				if !found {
					location := ThemeLayoutLocation{
						Name: place,
						Title: title,
					}
					if v, err := s.Html(); err == nil {
						fragment := html.UnescapeString(v)
						for _, line := range strings.Split(fragment, "\n") {
							reader := csv.NewReader(strings.NewReader(strings.TrimSpace(line)))
							reader.Comma = ' ' // space
							if cells, err := reader.Read(); err == nil {
								if len(cells) > 6 {
									if res := regexp.MustCompile(`plugins/([^/]+)/index.html`).FindStringSubmatch(cells[2]); len(res) > 1 {
										name := res[1]
										instance := ThemeLayoutPluginInstanceView{
											Name: name,
										}
										pairs := cells[7: len(cells) - 3]
										var left string
										for i, pair := range pairs {
											if i % 2 == 0 {
												left = pair
											}else{
												var typ, description string
												for _, plugin := range plugins {
													if plugin.Name == name {
														for _, param := range plugin.Params {
															if param.Name == left {
																typ = param.Type
																description = param.Description
																break
															}
														}
														break
													}
												}
												param := PluginParamView{
													Type: typ,
													Name: left,
													Description: description,
												}

												if v, err := url.QueryUnescape(pair); err == nil {
													param.Value = reRemoveRelative.ReplaceAllString(v, "")
												}else{
													param.Value = pair
												}

												instance.Params = append(instance.Params, param)
											}
										}
										location.Plugins = append(location.Plugins, instance)
									}
								}
							}else{
								fmt.Println(err)
								return
							}
						}
					}
					layout.Locations = append(layout.Locations, location)
				}
			})
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}

		layout.Size = fi.Size()
		layout.ModTime = fi.ModTime()
	} else {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}

	c.Status(http.StatusOK)
	return c.JSON(layout)
}

// @security BasicAuth
// PutThemeLayout godoc
// @Summary Put theme layout
// @Accept json
// @Produce json
// @Success 200 {object} ThemeLayoutView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/themes/{name}/layouts/{any} [put]
// @Tags theme
func putThemeLayoutHandler(c *fiber.Ctx) error {
	var name string
	if name = c.Params("name"); name == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Name not set"})
	}

	p := strings.Replace(string(c.Request().URI().Path()), "/api/v1/themes/" + name + "/layouts", "", 1)
	p = strings.Replace(p, "../", "", -1)

	var request ThemeLayoutView
	if err := c.BodyParser(&request); err != nil {
		return err
	}

	filepath := path.Join(dir, "hugo", "themes", name, "layouts", p)

	if _, err := os.Stat(path.Dir(filepath)); err != nil {
		if err = os.MkdirAll(filepath, 0755); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}

	bts, err := ioutil.ReadFile(filepath)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}

	var content = string(bts)

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bts))
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}

	for _, location := range request.Locations {
		doc.Find("[data-place=" + location.Name + "]").Each(func(i int, s *goquery.Selection) {
			var codes []string
			for _, plugin := range location.Plugins {
				var params []string
				for _, param := range plugin.Params {
					switch param.Value.(type) {
					case int:
						params = append(params, fmt.Sprintf(`"%s" %v`, param.Name, param.Value))
					case float64:
						params =  append(params, fmt.Sprintf(`"%s" %v`, param.Name, param.Value))
					default:
						if param.Value != nil {
							t := &url.URL{Path: fmt.Sprintf("%v", param.Value)}
							params = append(params, fmt.Sprintf(`"%s" "%s"`, param.Name, t.String()))
						}else{
							params = append(params, fmt.Sprintf(`"%s" "%s"`, param.Name, ""))
						}
					}
				}
				codes = append(codes, fmt.Sprintf(`{{ partial "plugins/%s/index.html" ( dict "context" . %s ) }}`, plugin.Name, strings.Join(params, " ")))
			}
			//
			if a, err := goquery.OuterHtml(s); err == nil {
				s.SetHtml(strings.Join(codes, "\n"))
				b, _ := goquery.OuterHtml(s)
				content = strings.Replace(content, html.UnescapeString(a), html.UnescapeString(b), 1)
			}
		})
	}

	if err := ioutil.WriteFile(filepath, []byte(content), 0755); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}

	request.Path = path.Dir(p)
	request.Name = path.Base(p)

	if fi, err := os.Stat(filepath); err == nil {
		request.Size = fi.Size()
		request.ModTime = fi.ModTime()
	} else {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}

	c.Status(http.StatusOK)
	return c.JSON(request)
}

// @security BasicAuth
// DeleteThemeLayout godoc
// @Summary Delete theme layout
// @Accept json
// @Produce json
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/themes/{name}/layouts/{any} [delete]
// @Tags theme
func delThemeLayoutHandler(c *fiber.Ctx) error {
	var name string
	if name = c.Params("name"); name == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Name not set"})
	}

	p := strings.Replace(string(c.Request().URI().Path()), "/api/v1/themes/"+name+"/layouts", "", 1)
	p = strings.Replace(p, "../", "", -1)

	filepath := path.Join(dir, "hugo", "themes", name, "layouts", p)
	if err := os.Remove(filepath); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}

	c.Status(http.StatusOK)
	return c.JSON(HTTPMessage{"OK"})
}

type PluginView struct {
	Name string `json:",omitempty"`
	Size int64
	Params []PluginParamView `json:",omitempty"`
	ModTime time.Time
}

type PluginsView []PluginsView
