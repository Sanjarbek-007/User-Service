package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"google_docs_user/api/email"
	"google_docs_user/api/token"
	pb "google_docs_user/genproto/user"
	"google_docs_user/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// @Summary      Register a new user
// @Description  This endpoint registers a new user by taking user details, hashing the password, and generating a confirmation code.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        user  body      models.Register  true  "User Registration Data"
// @Success      202   {object}  user.RegisterRes
// @Failure      400   {object}  string
// @Failure      500   {object}  string
// @Router       /auth/register [post]
func (h Handler) Register(c *gin.Context) {
	req := models.Register{}

	err := json.NewDecoder(c.Request.Body).Decode(&req)
	if err != nil {
		h.Log.Error(fmt.Sprintf("bodydan malumotlarni olishda xatolik: %v", err))
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashpassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.Log.Error(fmt.Sprintf("Pasworni hashlashda xatolik: %v", err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	req.Password = string(hashpassword)

	source := rand.NewSource(time.Now().UnixNano())
	myRand := rand.New(source)

	randomNumber := myRand.Intn(900000) + 100000

	req1 := pb.RegisterReq{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Password:  req.Password,
		Code:      int64(randomNumber),
	}

	resp, err := h.User.Register(c, &req1)
	if err != nil {
		h.Log.Error(fmt.Sprintf("Foydalanuvchi malumotlarni bazga yuborishda xatolik: %v", err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	myString := strconv.Itoa(randomNumber)
	err = email.SendCode(req1.Email, myString)
	if err != nil {
		h.Log.Error("Emailga xabar yuborishda xatolik", "error", err.Error())
		c.AbortWithStatusJSON(500, gin.H{
			"Emailga xabar yuborishda xatolik": err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, resp)
}

// @Summary      Login a user
// @Description  This endpoint logs in a user by checking the credentials and generating JWT tokens.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        credentials  body      user.LoginReq  true  "User Login Data"
// @Success      202   {object}  string "Tokens"
// @Failure      400   {object}  string
// @Failure      401   {object}  string
// @Failure      500   {object}  string
// @Router       /auth/login [post]
func (h Handler) LoginUser(c *gin.Context) {
	req := pb.LoginReq{}

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, err)
		return
	}

	req1 := pb.GetUSerByEmailReq{
		Email: req.Email,
	}

	user, err := h.User.GetUSerByEmail(c, &req1)
	if err != nil {
		h.Log.Error(fmt.Sprintf("GetbyUserda xatolik: %v", err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}

	if err = bcrypt.CompareHashAndPassword([]byte(user.User.Password), []byte(req.Password)); err != nil {
		log.Printf("Password comparison failed: %v", err)
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token := token.GenerateJWT(&pb.User{
		Id:       user.User.Id,
		Email:    req.Email,
		Password: req.Password,
		Role:     user.User.Role,
	})

	_, err = h.User.StoreRefreshToken(context.Background(), &pb.StoreRefreshTokenReq{
		UserId:  user.User.Id,
		Refresh: token.Refresh,
	})

	if err != nil {
		h.Log.Error(fmt.Sprintf("storefreshtokenda xatolik: %v", err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusAccepted, token)
}

// ConfirmationRegister godoc
// @Summary      Confirm user registration
// @Description  This endpoint confirms user registration by verifying the email and confirmation code.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        email  path      string  true  "User Email"
// @Param        code   path      string  true  "Confirmation Code"
// @Success      200    {object}  user.ConfirmationRegisterRes
// @Failure      400    {object}  string
// @Failure      500    {object}  string
// @Router       /auth/confirm/{email}/{code} [get]
func (h Handler) ConfirmationRegister(c *gin.Context) {
	codestr := c.Param("code")

	code, err := strconv.ParseInt(codestr, 10, 64)
	if err != nil {
		h.Log.Error(fmt.Sprintf("Kod stringdan int64 ga o'tkazishda xatolik: %v", err))
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Kod noto'g'ri formatda"})
		return
	}

	fmt.Println(code)

	req := pb.ConfirmationRegisterReq{
		Email: c.Param("email"),
		Code:  code,
	}

	res, err := h.User.ConfirmationRegister(c, &req)
	if err != nil {
		h.Log.Error("ConfirmationRegister funksiyasiga yuborishda xatolik.", "error", err)
		c.AbortWithStatusJSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, res)
}

// @Summary      Update user password
// @Description  This endpoint updates the user password after validating the old password.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        old_password  path      string  true  "Old Password"
// @Param        new_password  path      string  true  "New Password"
// @Param        email         path      string  true  "User Email"
// @Success      200    {object}  user.UpdatePasswordRes
// @Failure      401    {object}  string
// @Failure      500    {object}  string
// @Router       /auth/update_password/{email}/{old_password}/{new_password} [put]
func (h Handler) UpdatePassword(c *gin.Context) {
	req := pb.UpdatePasswordReq{
		OldPassword: c.Param("old password"),
		NewPassword: c.Param("new password"),
		Email:       c.Param("email"),
	}

	req1 := pb.GetUSerByEmailReq{
		Email: req.Email,
	}

	resemail, err := h.User.GetUSerByEmail(c, &req1)
	if err != nil {
		h.Log.Error("Email buyicha malumotlarni olishda xatolik", "error", err.Error())
		c.AbortWithStatusJSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	if err = bcrypt.CompareHashAndPassword([]byte(resemail.User.Password), []byte(req.OldPassword)); err != nil {
		log.Printf("Password comparison failed: %v", err)
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	hashpassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		h.Log.Error(fmt.Sprintf("Pasworni hashlashda xatolik: %v", err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	req.NewPassword = string(hashpassword)

	res, err := h.User.UpdatePassword(c, &req)
	if err != nil {
		h.Log.Error("UpdatePassword funksiyasiga malumot yuboishda xatolik.", "error", err.Error())
		c.AbortWithStatusJSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, res)
}

// @Summary      Reset user password
// @Description  This endpoint sends a password reset email to the specified email address.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        email  path      string  true  "User Email"
// @Success      200    {object}  user.ResetPasswordRes
// @Failure      400    {object}  string
// @Failure      500    {object}  string
// @Router       /auth/reset_password/{email} [get]
func (h Handler) ResetPassword(c *gin.Context) {
	req := pb.ResetPasswordReq{
		Email: c.Param("email"),
	}

	res, err := email.Email(req.Email)
	if err != nil {
		h.Log.Error("Email ga xabar yuuborishda xatolik.", "error", err.Error())
		c.AbortWithStatusJSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, res)
}

// @Summary      Confirm new password
// @Description  This endpoint confirms the new password by updating the user's password after validation.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        confirmation  body      user.ConfirmationReq  true  "Confirmation Data"
// @Success      200    {object}  user.ConfirmationResponse
// @Failure      400    {object}  string
// @Failure      500    {object}  string
// @Router       /auth/confirmation_password [post]
func (h Handler) ConfirmationPassword(c *gin.Context) {
	req := pb.ConfirmationReq{}

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, err)
		return
	}

	hashpassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		h.Log.Error(fmt.Sprintf("Pasworni hashlashda xatolik: %v", err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	req.NewPassword = string(hashpassword)

	res, err := h.User.ConfirmationPassword(c, &req)
	if err != nil {
		h.Log.Error("ConfirmationPassword funksiyasiga malumot yuborishda xatolik", "error", err.Error())
		c.AbortWithStatusJSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, res)
}

// @Summary      Update user role
// @Description  This endpoint updates the user's role based on the provided email and new role.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        email  path      string  true  "User Email"
// @Param        role   path      string  true  "New User Role"
// @Success      200    {object}  user.UpdateRoleRes
// @Failure      400    {object}  string
// @Failure      500    {object}  string
// @Router       /auth/update_role/{email}/{role} [put]
func (h Handler) UpdateRole(c *gin.Context) {
	req := pb.UpdateRoleReq{
		Email: c.Param("email"),
		Role:  c.Param("role"),
	}

	res, err := h.User.UpdateRole(c, &req)
	if err != nil {
		h.Log.Error("Update role ga malumot yuborishda xatolik", "error", err.Error())
		c.AbortWithStatusJSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, res)
}

// @Summary Upload Photo
// @Description Api for upload a new photo
// @Tags auth_media
// @Accept multipart/form-data
// @Param        email  path      string  true  "User Email"
// @Param file formData file true "createUserModel"
// @Success 200 {object} string
// @Failure 400 {object} string
// @Failure 500 {object} string
// @Router /auth/products/media/{email} [post]
func (h Handler) UploadMedia(c *gin.Context) {
	var file models.File
	email := c.Param("email")

	err := c.ShouldBind(&file)
	if err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	fileUrl := filepath.Join("./media", file.File.Filename)

	err = c.SaveUploadedFile(&file.File, fileUrl)
	if err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	fileExt := filepath.Ext(file.File.Filename)

	newFile := uuid.NewString() + fileExt

	minioClient, err := minio.New("3.75.208.130:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("minio", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	bucketName := "photos"

	exists, err := minioClient.BucketExists(context.Background(), bucketName)
	if err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	if !exists {
		err = minioClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
		if err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}
	}

	_, err = minioClient.FPutObject(context.Background(), bucketName, newFile, fileUrl, minio.PutObjectOptions{
		ContentType: "image/jpeg",
	})
	if err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	policy := fmt.Sprintf(`{
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": {
                    "AWS": ["*"]
                },
                "Action": ["s3:GetObject"],
                "Resource": ["arn:aws:s3:::%s/*"]
            }
        ]
    }`, bucketName)

	err = minioClient.SetBucketPolicy(context.Background(), bucketName, policy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"Message": err.Error(),
		})
		log.Println(err.Error())
		return
	}

	objUrl, err := minioClient.PresignedGetObject(context.Background(), bucketName, newFile, time.Hour*24, nil)
	if err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	req := pb.ImageReq{Image: objUrl.String(), Email: email}

	_, err = h.User.ProfileImage(c, &req)
	if err != nil {
		h.Log.Error("Email buyicha malumotlarni olishda xatolik", "error", err.Error())
		c.AbortWithStatusJSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	maduUrl := fmt.Sprintf("http://3.75.208.130:9000/photos/%s", newFile)

	c.JSON(201, gin.H{
		"obj":     objUrl.String(),
		"madeUrl": maduUrl,
	})
}

// @Summary      get user by Email
// @Description  This endpoint gets user's information with his email.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        email  path      string  true  "User Email"
// @Success      200    {object}  user.GetUserResponse
// @Failure      400    {object}  string
// @Failure      500    {object}  string
// @Router       /auth/GetuserByEmail/{email} [get]
func (h Handler) GetUSerByEmail(c *gin.Context) {
	req := pb.GetUSerByEmailReq{
		Email: c.Param("email"),
	}

	res, err := h.User.GetUSerByEmail(c, &req)
	if err != nil {
		h.Log.Error("erorr while searich user by email", "error", err.Error())
		c.AbortWithStatusJSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, res)
}

// @Summary      Update user role
// @Description  This endpoint updates the user based on the provided email and new role.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        email  path      string  true  "User Email"
// @Param        firstname   path      string  true  "Firstname"
// @Param        lastname   path      string  true  "New Lastname"
// @Success      200    {object}  user.UpdateUserRespose
// @Failure      400    {object}  string
// @Failure      500    {object}  string
// @Router       /auth/UpdateUser/{email}/{firstname}/{lastname} [put]
func (h Handler) UpdateUser(c *gin.Context) {
	req := pb.UpdateUserRequest{
		Email:     c.Param("email"),
		FirstName: c.Param("firstname"),
		LastName:  c.Param("lastname"),
	}

	res, err := h.User.UpdateUser(c, &req)
	if err != nil {
		h.Log.Error("error while updating user", "error", err.Error())
		c.AbortWithStatusJSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, res)
}

// @Summary      Delete user
// @Description  This endpoint deletes the user's profile.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        user_id  path      string  true  "User Email"
// @Success      200    {object}  user.DeleteUserr
// @Failure      400    {object}  string
// @Failure      500    {object}  string
// @Router       /auth/DeleteUser/{user_id} [delete]
func (h Handler) DeleteUser(c *gin.Context) {
	req := pb.UserId{
		Id: c.Param("user_id"),
	}

	res, err := h.User.DeleteUser(c, &req)
	if err != nil {
		h.Log.Error("error while deleting user", "error", err.Error())
		c.AbortWithStatusJSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, res)
}

// @Summary      get all users
// @Description  This endpoint gets all users informations.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        limit  path      string  true  "limit"
// @Param        offset path     string  true  "offset"
// @Success      200    {object}  user.GetAllUsersRes
// @Failure      400    {object}  string
// @Failure      500    {object}  string
// @Router       /auth/GetAllUsers/{limit}/{offset} [get]
func (h Handler) GetAllUsers(c *gin.Context) {
	limitStr := c.Param("limit")
	OffsetStr := c.Param("offset")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid offset: %s", OffsetStr)})
		return
	}

	offset, err := strconv.Atoi(OffsetStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid offset: %s", OffsetStr)})
		return
	}

	req := pb.GetAllUsersReq{
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	res, err := h.User.GetallUsers(c, &req)
	if err != nil {
		h.Log.Error("erorr while searich getall users", "error", err.Error())
		c.AbortWithStatusJSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, res)
}

// @Summary      Health check
// @Description  This endpoint checks if the server is healthy.
// @Tags         health
// @Accept       json
// @Produce      json
// @Success      200    {object}  string  "OK"
// @Router       /auth/health [get]
func (h Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "OK"})
}
