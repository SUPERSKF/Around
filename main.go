package main

import (
	elastic "gopkg.in/olivere/elastic.v3"
	"fmt"
	"net/http"
	"encoding/json"
	"log"
	"strconv"
	"reflect"
	"github.com/pborman/uuid"
)

type Location struct{
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}
type Post struct {
	// `json:"user"` is for the json parsing of this User field. Otherwise, by default it's 'User'.
	User     string `json:"user"`
	Message  string  `json:"message"`
	Location Location `json:"location"`
}

const (
	INDEX = "around"
	TYPE = "post"
	DISTANCE = "200km"
	// Needs to update
	//PROJECT_ID = "around-xxx"
	//BT_INSTANCE = "around-post"
	// Needs to update this URL if you deploy it to cloud.
	ES_URL = "http://35.197.55.145:9200"
)

func main() {
	client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err!= nil{
		panic(err)
		return
	}

	exists, err:= client.IndexExists(INDEX).Do()
	if err!=nil{
		panic(err)
	}
	if !exists{
		mapping:=`{
			"mappings":{
				"post":{
					"properties":{
						"location":{
							"type":"geo_point"
						}
					}
				}
			}
		}`

		_, err :=client.CreateIndex(INDEX).Body(mapping).Do()
		if err!= nil{
			panic(err)
		}
	}
	fmt.Println("started-service")
	http.HandleFunc("/post", handlerPost)
	http.HandleFunc("/search", handlerSearch)
	log.Fatal(http.ListenAndServe(":8080",nil))
}

func handlerPost(w http.ResponseWriter, r *http.Request) {
	// Parse from body of request to get a json object.
	fmt.Println("Received one post request")
	decoder := json.NewDecoder(r.Body)
	var p Post
	if err := decoder.Decode(&p); err != nil {
		panic(err)
		return
	}
	id:=uuid.New()
	saveToES(&p, id)
	fmt.Fprintf(w, "Post received: %s\n", p.Message)
}
func saveToES(p *Post, id string){
	es_client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err!= nil{
		panic(err)
		return
	}
	_, err = es_client.Index().
		Index(INDEX).
		Type(TYPE).
		Id(id).
		BodyJson(p).
		Refresh(true).
		Do()
	if err!=nil{
		panic(err)
		return
	}
	fmt.Printf("Post is saved to Index: %s\n", p.Message)

}


func handlerSearch(w http.ResponseWriter, r *http.Request){
	fmt.Println("Received one search request")
	lat,_:=strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lon,_:=strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	ran :=DISTANCE
	if val:=r.URL.Query().Get("range"); val!=""{
		ran = val + "km"
	}
	fmt.Println("range is", ran)

	fmt.Printf("Search received: %f%f%s\n", lat, lon, ran)
	client, err :=elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		panic(err)
		return
	}
	q := elastic.NewGeoDistanceQuery("location")
	q = q.Distance(ran).Lat(lat).Lon(lon)

	searchResult, err := client.Search().
		Index(INDEX).
		Query(q).
		Pretty(true).
		Do()
	if err != nil {
		// Handle error
		panic(err)
	}

	var typ Post
	var ps []Post

	for _,item:=range searchResult.Each(reflect.TypeOf(typ)){
		p:=item.(Post)
		fmt.Printf("Post by %s: %s at lat %v and lon %v\n", p.User, p.Message, p.Location.Lat, p.Location.Lon)
		ps = append(ps,p)
	}

	js, err:=json.Marshal(ps)
	if err!=nil{
		panic(err)
		return
	}
	w.Header().Set("Content-Type","application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(js)
}
