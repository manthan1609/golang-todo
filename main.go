package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/thedevsaddam/renderer"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var rnd *renderer.Render
var db *mgo.Database

const (
	hostName 		string = "localhost:27017"
	dbName 			string = "golang_todo"
	collectionName 	string = "todo"
	port 			string = ":9000"
)

type (
	todoModel struct {
		ID 			bson.ObjectId 	`bson:"_id,omitempty"`
		Title 		string			`bson:"title"`
		Completed 	bool			`bson:"completed"`
		CreatedAt	time.Time		`bson:"createdAt"`
	}

	todo struct {
		ID 			string 	`json:"id,omitempty"`
		Title 		string			`json:"title"`
		Completed 	bool			`json:"completed"`
		CreatedAt	time.Time		`json:"created_at"`
	}
)

func init()  {
	rnd = renderer.New()

	sess,err := mgo.Dial(hostName)
	
	checkErr(err)
	
	sess.SetMode(mgo.Monotonic,true)

	db = sess.DB(dbName)
}

func checkErr(err error)  {
	if err!=nil {
		log.Fatal(err)
	}
}

// Handlers
func todoHandlers() http.Handler  {
	rg := chi.NewRouter()

	rg.Group(func(r chi.Router) {
		r.Get("/",fetchTodos)
		r.Post("/",createTodo)
		r.Put("/{id}",updateTodo)
		r.Delete("/{id}",deleteTodo)

	})

	return rg
}

func fetchTodos(w http.ResponseWriter,r *http.Request)  {
	todos := []todoModel{}

	if err := db.C(collectionName).Find(bson.M{}).All(&todos);err!=nil {
		rnd.JSON(w,http.StatusProcessing,renderer.M{
			"message":"failed to fetch todos",
			"error":err,
		})
		return
	}

	todoList := []todo{}

	for _,t := range todos{
		todoList = append(todoList,todo{
			ID: t.ID.Hex(),
			Title: t.Title,
			Completed: t.Completed,
			CreatedAt: t.CreatedAt,
		})
	}

	rnd.JSON(w,http.StatusOK,renderer.M{
			"data":todoList,
	})
}

func createTodo(w http.ResponseWriter,r *http.Request)  {
	var t todo

	if err:= json.NewDecoder(r.Body).Decode(&t);err!=nil {
		rnd.JSON(w,http.StatusBadRequest,renderer.M{
			"message":"invalid data",
			"error":err,
		})
		return
	}
	if t.Title=="" {
		rnd.JSON(w,http.StatusBadRequest,renderer.M{
			"message":"title is required",
		})
		return
	}

	todo := todoModel{
		ID: bson.NewObjectId(),
		Title: t.Title,
		Completed: false,
		CreatedAt: time.Now(),
	}

	if err := db.C(collectionName).Insert(&todo);err!=nil {
		rnd.JSON(w,http.StatusInternalServerError,renderer.M{
			"message":"database error",
			"error":err,
		})
		return
	}

	rnd.JSON(w,http.StatusCreated,renderer.M{
			"message":"todo created successfully",
			"todo_id":todo.ID.Hex(),
	})
}

func deleteTodo(w http.ResponseWriter,r *http.Request)  {
	id := strings.TrimSpace(chi.URLParam(r,"id"))

	if !bson.IsObjectIdHex(id) {
		rnd.JSON(w,http.StatusBadRequest,renderer.M{
			"message":"invalid id",
		})
		return
	}

	
	if err := db.C(collectionName).Remove(bson.M{
		"_id":id,
	});err==nil {
		rnd.JSON(w,http.StatusInternalServerError,renderer.M{
			"message":"database error",
			"error":err,
		})
		return
	}

	rnd.JSON(w,http.StatusOK,renderer.M{
			"message":"todo deleted successfully",
	})
}

func updateTodo(w http.ResponseWriter,r *http.Request)  {
	id := strings.TrimSpace(chi.URLParam(r,"id"))
	var t todo


	if !bson.IsObjectIdHex(id) {
		rnd.JSON(w,http.StatusBadRequest,renderer.M{
			"message":"invalid id",
		})
		return
	}


	if err:= json.NewDecoder(r.Body).Decode(&t);err!=nil {
		rnd.JSON(w,http.StatusBadRequest,renderer.M{
			"message":"invalid data",
			"error":err,
		})
		return
	}
	if t.Title=="" {
		rnd.JSON(w,http.StatusBadRequest,renderer.M{
			"message":"title is required",
		})
		return
	}


	if err := db.C(collectionName).UpdateId(
		bson.ObjectIdHex(id),
		todoModel{
			Title: t.Title,
			Completed: t.Completed,
		},
	);err!=nil {
		rnd.JSON(w,http.StatusInternalServerError,renderer.M{
			"message":"database error",
			"error":err,
		})
		return
	}


	rnd.JSON(w,http.StatusOK,renderer.M{
			"message":"todo updated successfully",
	})	
}


func homeHandler(w http.ResponseWriter,r *http.Request)  {
	err := rnd.Template(w,http.StatusOK,[]string{"static/home.tpl"},nil)
	checkErr(err)
}


func main() {
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan,os.Interrupt)
	r := chi.NewRouter()

	r.Use(middleware.Logger)

	r.Get("/",homeHandler)
	r.Mount("/todo",todoHandlers())

	srv := &http.Server{
		Addr: port,
		Handler: r,
		ReadTimeout: 60*time.Second,
		WriteTimeout: 60*time.Second,
		IdleTimeout: 60*time.Second,
	}

	go func ()  {
		log.Println("Listening on port",port)
		if err:= srv.ListenAndServe(); err!=nil{
			log.Println(err)
		}
	}()

	<-stopChan
	log.Println("Shutting down server")

	ctx ,cancel := context.WithTimeout(context.Background(),5*time.Second)
	srv.Shutdown(ctx)

	log.Println("Server Gracefully Shutdown")
	defer cancel()
}

