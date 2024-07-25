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
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var rnd *renderer.Render
var db *mongo.Database
var collection *mongo.Collection

const (
	hostName 		string = "mongodb://localhost:27017/"
	dbName 			string = "golang_todo"
	collectionName 	string = "todo"
	port 			string = ":9000"
)

type (
	todoModel struct {
		ID 			primitive.ObjectID 	`bson:"_id,omitempty"`
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

	// ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	clientOption := options.Client().ApplyURI(hostName)

	// defer cancel()
	client, err := mongo.Connect(context.TODO(),clientOption)
	
	checkErr(err)

	db = client.Database(dbName)
	collection = db.Collection(collectionName)

	if collection!=nil {
		log.Println("COllection Ready")
	}else{
		log.Println("COllection Not Ready")
	}

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

	cur,err := collection.Find(context.Background(),bson.D{{}})

	if cur==nil {
		rnd.JSON(w,http.StatusProcessing,renderer.M{
			"message":"failed to fetch todos",
			"error":err,
		})
		return
	}


	for cur.Next(context.Background()){
		var t todoModel

		err := cur.Decode(&t)

		if err!=nil {
			log.Fatal(err)
		}

		todos = append(todos, t)
	}

	if err!=nil {
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

	defer cur.Close(context.Background())
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

	res,err :=collection.InsertOne(context.Background(),bson.M{
		"title":t.Title,
		"completed":t.Completed,
		"createdAt":t.CreatedAt,
	})

	if err!=nil {
		rnd.JSON(w,http.StatusInternalServerError,renderer.M{
			"message":"database error",
			"error":err,
		})

		return
	}

	rnd.JSON(w,http.StatusCreated,renderer.M{
			"message":"todo created successfully",
			"todo_id":res.InsertedID,
	})
}

func deleteTodo(w http.ResponseWriter,r *http.Request)  {
	id := strings.TrimSpace(chi.URLParam(r,"id"))

	
	objId,err := primitive.ObjectIDFromHex(id)
	

	if  err!=nil{
		rnd.JSON(w,http.StatusBadRequest,renderer.M{
			"message":"invalid id",
		})
		return
	}

	
	res,err := collection.DeleteOne(context.Background(),bson.M{
		"_id":objId,
	})

	if err!=nil {
		rnd.JSON(w,http.StatusInternalServerError,renderer.M{
			"message":"database error",
			"error":err,
		})
		return
	}


	rnd.JSON(w,http.StatusOK,renderer.M{
		"message":"todo deleted successfully",
		"deleted":res.DeletedCount,
	})
	
}

func updateTodo(w http.ResponseWriter,r *http.Request)  {
	id := strings.TrimSpace(chi.URLParam(r,"id"))
	objId,err := primitive.ObjectIDFromHex(id)

	if err!=nil {
		rnd.JSON(w,http.StatusBadRequest,renderer.M{
			"message":"invalid id",
		})
		return
	}

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

	res,err := collection.UpdateOne(context.Background(),bson.M{
		"_id":objId,
	},bson.M{
		"$set":bson.M{
			"title":t.Title,
			"completed":t.Completed,
		},
	})

	if err!=nil {
		rnd.JSON(w,http.StatusInternalServerError,renderer.M{
			"message":"database error",
			"error":err,
		})

		return
	}

	rnd.JSON(w,http.StatusOK,renderer.M{
			"message":"todo updated successfully",
			"updated":res.ModifiedCount,
	})
	
}


func homeHandler(w http.ResponseWriter,r *http.Request)  {
	err := rnd.Template(w,http.StatusOK,[]string{"static/home.tpl"},nil)
	checkErr(err)
}


func main() {
	stopChan := make(chan os.Signal,1)
	signal.Notify(stopChan, os.Interrupt)


	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", homeHandler)

	r.Mount("/todo", todoHandlers())

	srv := &http.Server{
		Addr:         port,
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Println("Listening on port ", port)
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("listen: %s\n", err)
		}
	}()

	<-stopChan
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	srv.Shutdown(ctx)
	defer cancel()
	log.Println("Server gracefully stopped!")
}



