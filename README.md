# FChat: Free Chat

Simple chat server for my students. Written using Golang.

## Getting Started
You don't need to install Go on your local machine.
To run with Docker just cd into directory and run 
`docker-compose start `

## Routes 
To send message make a POST request to 
```
http://{server_address}/send/{room_name}/
```
`room_name` can be anything

in your post request include 
`Name` field and `Message`. We accept JSON :) 


To get messages make a POST or GET request to 
```
http://{server_address}/get/{room_name}/
```
`room_name` can be anything

in your post request include 
`Offset` field and `Limit`. Here we also accept JSON :) 

## Built With

* Golang net/http
* Postgres
* That's all no fancy frameworks required here

## Authors

* **Arick Vigas** - *Initial work* - [iarickvigasi](https://github.com/iarickvigasi)

## License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details

