package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Post struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	UserID int    `json:"userId"`
}

type User struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type Comment struct {
	ID     int    `json:"id"`
	PostID int    `json:"postId"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Body   string `json:"body"`
}

func CreatePost(c *gin.Context) {
	var post Post
	if err := c.ShouldBindJSON(&post); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Aquí iría la lógica para crear el post
	c.JSON(http.StatusCreated, gin.H{
		"message": "Post creado exitosamente",
		"post":    post,
	})
}

func CreateUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Aquí iría la lógica para crear el usuario
	c.JSON(http.StatusCreated, gin.H{
		"message": "Usuario creado exitosamente",
		"user":    user,
	})
}

func CreateComment(c *gin.Context) {
	var comment Comment
	if err := c.ShouldBindJSON(&comment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Aquí iría la lógica para crear el comentario
	c.JSON(http.StatusCreated, gin.H{
		"message": "Comentario creado exitosamente",
		"comment": comment,
	})
}

// Nuevos handlers GET con parámetros por URL
func GetPostByID(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Post obtenido por ID",
		"id":      id,
		"post":    Post{ID: 1, Title: "Post de prueba", Body: "Contenido de prueba", UserID: 1},
	})
}

func GetUserByID(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Usuario obtenido por ID",
		"id":      id,
		"user":    User{ID: 1, Name: "Usuario de prueba", Username: "testuser", Email: "test@example.com"},
	})
}

func GetCommentByID(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"message": "Comentario obtenido por ID",
		"id":      id,
		"comment": Comment{ID: 1, PostID: 1, Name: "Comentarista", Email: "comment@example.com", Body: "Comentario de prueba"},
	})
}
