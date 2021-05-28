package main

import (
	"context"
	"errors"
	"html/template"
	"log"
	"net/http"
	"regexp"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Page represents single wiki Page
type Page struct {
	Title string
	Body  []byte
}

func (p *Page) save() error {

	filter := bson.D{primitive.E{Key: "title", Value: p.Title}}
	_, err := pagesCollection.ReplaceOne(ctx, filter,
		bson.D{
			primitive.E{Key: "title", Value: p.Title},
			primitive.E{Key: "body", Value: p.Body},
		},
	)

	return err
}

func deletePage(title string) error {

	filter := bson.D{primitive.E{Key: "title", Value: title}}
	_, err := pagesCollection.DeleteOne(ctx, filter)

	return err
}

func loadPage(title string) (*Page, error) {

	var result *Page
	filter := bson.D{primitive.E{Key: "title", Value: title}}
	dbErr := pagesCollection.FindOne(ctx, filter).Decode(&result)

	if dbErr != nil {
		return nil, errors.New("Page not found")
	}

	return result, nil
}

func listPages() ([]string, error) {

	cur, err := pagesCollection.Find(ctx, bson.D{})

	names := []string{}

	if err != nil {
		log.Fatal(err)
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var result Page
		err := cur.Decode(&result)
		if err != nil {
			log.Fatal(err)
		}

		names = append(names, result.Title)
	}

	return names, nil
}

var validPath = regexp.MustCompile("^/(edit|save|view|delete)/([a-zA-Z0-9]+)$")

func getTitle(w http.ResponseWriter, r *http.Request) (string, error) {
	m := validPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.NotFound(w, r)
		return "", errors.New("invalid Page Title")
	}
	return m[2], nil // The title is the second subexpression.
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}
	renderPageTemplate(w, "view", p)
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	pages, err := listPages()
	if err != nil {
		http.Redirect(w, r, "/list", http.StatusFound)
		return
	}
	err = templates.ExecuteTemplate(w, "list.html", pages)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
	renderPageTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func deleteHandler(w http.ResponseWriter, r *http.Request, title string) {

	err := deletePage(title)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/list", http.StatusFound)
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

var templates = template.Must(
	template.ParseFiles(
		"Templates/edit.html",
		"Templates/view.html",
		"Templates/list.html",
	),
)

func renderPageTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var db *mongo.Database
var pagesCollection *mongo.Collection
var ctx = context.TODO()

func main() {

	dbOptions := options.Client().ApplyURI("mongodb://localhost:27017/")
	dbConnection, err := mongo.Connect(ctx, dbOptions)
	if err != nil {
		log.Fatal(err)
	}

	err = dbConnection.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	db = dbConnection.Database("golang")
	pagesCollection = db.Collection("Pages")

	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/delete/", makeHandler(deleteHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	http.HandleFunc("/list", listHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
