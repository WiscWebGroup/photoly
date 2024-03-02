package org.chengbing.controller;


import com.alibaba.fastjson.JSONObject;
import com.alibaba.fastjson.TypeReference;
import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.fasterxml.jackson.databind.util.JSONPObject;
import net.glxn.qrgen.QRCode;
import org.chengbing.entity.Photo;
import org.chengbing.service.IPhotoService;
import org.chengbing.service.IUserService;
import org.chengbing.util.Result;
import org.chengbing.util.UserIdentity;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.core.io.InputStreamResource;
import org.springframework.core.io.InputStreamSource;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import org.springframework.web.multipart.MultipartFile;

import javax.annotation.Resource;
import javax.imageio.ImageIO;
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletRequestWrapper;
import javax.servlet.http.HttpServletResponse;
import java.awt.*;
import java.awt.image.BufferedImage;
import java.io.*;
import java.net.URLEncoder;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;

/**
 * <p>
 *  前端控制器
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@RestController
@RequestMapping("/photo")
public class PhotoController {
    @Resource
    IPhotoService service;

    @Resource
    UserIdentity verify;

    @Resource
    IUserService userService;

    @Value("${file.uploadFolder}")
    String uploadFolder;

    @Value("${server.path}")
    String serverPath;

    /**
     * Function to insert one photo into a namespace of current user's
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param file a file uploaded (photo file)
     * @param photo a string representation of a JSON serialized Photo object. Only "nsId" is required. "visibility"
     *              is optional.
     * @return a result of integer to indicate success or not. (1, 200) means successful.
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/insert")
    public Result<Integer> insertPhoto(HttpServletRequest request, MultipartFile file, @RequestParam String photo)
    {
        Photo photo1 = JSONObject.parseObject(photo, Photo.class);
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.insertPhoto(userId, file, photo1);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to insert multiple photos into (one or more) namespace of current user's
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param files files uploaded (photo files)
     * @param photosStr a string representation of a JSON serialized List of Photo object. Only "nsId" is required.
     *              "visibility" is optional.
     * example for photosStr:
     * '[{"nsId": 60}, {"nsId": 60, "visibility": 0}]'
     * @return a result of integer to indicate success or not. (>=1 (# of photos you uploaded), 200) means successful.
     * example:
     * {
     *     "t": 2,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/inserts")
    public Result<Integer> insertPhotos(HttpServletRequest request, MultipartFile[] files, @RequestParam String photosStr)
    {
        List<Photo> photos = JSONObject.parseObject(photosStr, new TypeReference<ArrayList<Photo>>(){});
        if (files.length != photos.size())
            return null;
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.insertPhotos(userId, files, photos);
        return change >= 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to delete a photo of current user's
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param photoId the ID of the photo to be deleted
     * @return a result of integer to indicate success or not. (1, 200) means successful.
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/delete")
    public Result<Integer> deletePhoto(HttpServletRequest request, Integer photoId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.deletePhoto(userId, photoId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to update a photo of current user's with its name or (and) visibility
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param photo a Photo object with its "photoId", "photoName", and "visibility" params included
     * example:
     * {
     *     "photoId": 645,
     *     "photoName": "hawaii-halo",
     *     "visibility": 1
     * }
     * @return a result of integer to indicate success or not. (1, 200) means successful.
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/update")
    public Result<Integer> updatePhotoNameAndVisibility(HttpServletRequest request, @RequestBody Photo photo)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updatePhotoNameAndVisibility(userId, photo);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to get all photos within a namespace
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param nsId a namespace's ID to query on
     * @return a result list of Photo objects within that namespace.
     * example:
     * {
     *     "t": [
     *         {
     *             "photoId": 111,
     *             "photoName": "hahaha",
     *             "uploadDate": "2023-02-12T14:55:00",
     *             "format": "jpg",
     *             "nsId": 65,
     *             "visibility": 1,
     *             "token": "c1101411-b003-41e6-b046-11111993c977",
     *             "photoUuid": "641c6881-0914-1111-9bca-1111f5e5310d",
     *             "userId": 321
     *         },
     *         {
     *             "photoId": 113,
     *             "photoName": "cuda",
     *             "uploadDate": "2023-02-12T15:08:33",
     *             "format": "png",
     *             "nsId": 65,
     *             "visibility": 0,
     *             "token": "1117952d-1111-4845-a663-63c11134151f",
     *             "photoUuid": "c4882783-4982-4b67-84b2-111164ef4071",
     *             "userId": 321
     *         },...
     *     ],
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getByNamespace")
    public Result<List<Photo>> queryPhotoByNamespace(HttpServletRequest request, Integer nsId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryPhotoByNamespace(userId, nsId), 200);
    }

    /**
     * Function to get all photos with one or more tags. Only the photo with all the tags on them gets selected.
     * For example, we have (photo1, photo2). photo1 is tagged with tagId 1 and 2, whereas photo2 is tagged with tagId 1.
     * If we are selecting from "tagIds": 1, 2; then only photo1 is selected. If we use "tagIds": 1; then photo 1 and 2
     * are both selected.
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param tagIds a list of integer (tag's IDs)
     * example for tagIds:
     * (Key: Value) - (tagIds: 57, 56)
     * @return a result list of Photo objects fulfill the tag requirement.
     * example:
     * {
     *     "t": [
     *         {
     *             "photoId": 65,
     *             "photoName": "Picture1",
     *             "uploadDate": "2023-02-12T15:08:34",
     *             "format": "png",
     *             "nsId": 61,
     *             "visibility": 0,
     *             "token": "11111117-3781-4987-bab2-111113cd5704",
     *             "photoUuid": "111111ec-405d-49e1-aafa-11111c802602",
     *             "userId": 321
     *         }
     *     ],...
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getByTags")
    public Result<List<Photo>> queryPhotoByTags(HttpServletRequest request, @RequestParam List<Integer> tagIds)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryPhotoByTags(userId, tagIds), 200);
    }

    /**
     * Function to get all photos within the gallery
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param gaId gallery's ID to query in
     * @return a result list of Photo objects which are associated with that gallery.
     * example:
     * {
     *     "t": [
     *         {
     *             "photoId": 315,
     *             "photoName": "Picture1",
     *             "uploadDate": "2023-02-12T15:08:34",
     *             "format": "png",
     *             "nsId": 40,
     *             "visibility": 0,
     *             "token": "11111117-3781-4987-bab2-111113cd5704",
     *             "photoUuid": "111111ec-405d-49e1-aafa-11111c802602",
     *             "userId": 321
     *         }
     *     ],...
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getByGallery")
    public Result<List<Photo>> queryPhotoByGallery(HttpServletRequest request, Integer gaId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryPhotoByGallery(userId, gaId), 200);
    }

    /**
     * Function to change the folder (namespace) of a photo
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param photoId the ID of the photo you would like to transfer
     * @param nsId the ID of the new namespace you want the photo go to
     * @return a result of integer to indicate success or not. (1, 200) means successful.
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/changeNamespace")
    public Result<Integer> changeNamespace(HttpServletRequest request, Integer photoId, Integer nsId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.changeNamespace(userId, photoId, nsId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to associate tags with a photo
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param photoId the ID of the photo you would like to transfer
     * @param tagIds a list of integer (tag's IDs) you want the photo to be associated with
     * @return a result of integer to indicate success or not. (>=1, 200) means successful.
     * example:
     * {
     *     "t": 2,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/addTags")
    public Result<Integer> addTags(HttpServletRequest request, Integer photoId, @RequestParam List<Integer> tagIds)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.addTags(userId, photoId, tagIds);
        return change >= 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to remove the association of a tag with a photo
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param photoId the ID of the photo you would like to de-associate with the tag
     * @param tagId the tag's ID you would like to de-associate with the photo
     * @return a result of integer to indicate success or not. (1, 200) means successful.
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/deleteTag")
    public Result<Integer> deleteTag(HttpServletRequest request, Integer photoId, Integer tagId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.deleteTag(userId, photoId, tagId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to remove the association of many tags with a photo
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param photoId the ID of the photo you would like to transfer
     * @param tagIds a list of integer (tag's IDs) you want the photo to be associated with
     * example:
     * (Key: Value) - (tagIds: 57, 56)
     * @return a result of integer to indicate success or not. (>=1, 200) means successful.
     * example:
     * {
     *     "t": 2,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/deleteTags")
    public Result<Integer> deleteTags(HttpServletRequest request, Integer photoId, @RequestParam List<Integer> tagIds)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = 0;
        for (Integer tagId : tagIds)
            change += deleteTag(request, photoId, tagId).getT();
        return change >= 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to add a photo to a gallery
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param photoId the ID of the photo you would like to add to the gallery
     * @param gaId the gallery you want the photo to be associated with
     * @return a result of integer to indicate success or not. (1, 200) means successful.
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/addToGallery")
    public Result<Integer> addToGallery(HttpServletRequest request, Integer photoId, Integer gaId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.addToGallery(userId, photoId, gaId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to remove a photo from a gallery (de connect them, instead of deleting photo or gallery)
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param photoId the ID of the photo you would like to remove from the gallery
     * @param gaId the gallery you want the photo to be de-associated with
     * @return a result of integer to indicate success or not. (1, 200) means successful.
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/deleteFromGallery")
    public Result<Integer> deleteFromGallery(HttpServletRequest request, Integer photoId, Integer gaId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.deleteFromGallery(userId, photoId, gaId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to get all tags of a photo given that photo's ID
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param photoId the ID of the photo you would like to query
     * @return a result of List of LinkedHashMap that contains the tag and photo information
     * example:
     * {
     *     "t": [
     *         {
     *             "photo_id": 111,
     *             "tag_name": "testTag2",
     *             "tag_id": 516
     *         },
     *         {
     *             "photo_id": 112,
     *             "tag_name": "testTag123",
     *             "tag_id": 517
     *         },...
     *     ],
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getTagByPhoto")
    public Result<List<LinkedHashMap<String, Object>>> selectTagByPhoto(HttpServletRequest request, Integer photoId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        List<LinkedHashMap<String, Object>> res = service.selectTagByPhoto(userId, photoId);
        if (res == null)
            return new Result<>(null, 400);
        else
            return new Result<>(res, 200);
    }

    /**
     * Function to get all galleries a photo associated with given that photo's ID
     *
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param photoId the ID of the photo you would like to query
     * @return a result of List of LinkedHashMap that contains the gallery and photo information
     * example:
     * {
     *     "t": [
     *         {
     *             "photo_id": 415,
     *             "ga_name": "testGallery123",
     *             "ga_id": 421
     *         }
     *     ],
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getGalleryByPhoto")
    public Result<List<LinkedHashMap<String, Object>>> selectGalleryByPhoto(HttpServletRequest request, Integer photoId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        List<LinkedHashMap<String, Object>> res = service.selectGalleryByPhoto(userId, photoId);
        if (res == null)
            return new Result<>(null, 400);
        else
            return new Result<>(res, 200);
    }

    /**
     * Function to render a photo using the photoId and the user's access token (given after login)
     *
     * @param token the token of the owner user of that photo
     * @param photoId the ID of the photo to be rendered
     * @return a ResponseEntity of Object (that image's bytes)
     */
    @GetMapping(value = "/render/{token}", produces = MediaType.IMAGE_JPEG_VALUE)
    public ResponseEntity<Object> renderImage(@PathVariable String token, Integer photoId) throws IOException {
        Integer userId = verify.verifyUserByToken(token);
        Photo photo = service.getById(photoId);
        String UUID = userService.getById(userId).getUuid();
        if (userId != null && photo != null && userId.equals(photo.getUserId()))
        {
            File file = new File(uploadFolder + System.getProperty("file.separator") + UUID + System.getProperty("file.separator") + photo.getPhotoUuid()
             + "." + photo.getFormat());
            FileInputStream inputStream = new FileInputStream(file);
            InputStreamSource inputStreamSource = new InputStreamResource(inputStream);
            return new ResponseEntity<>(inputStreamSource, new HttpHeaders(), HttpStatus.OK);
        }
        return null;
    }

    /**
     * Function to render a thumbnail version of the photo using the photoId and the user's access token (given after login)
     *
     * @param token the token of the owner user of that photo
     * @param photoId the ID of the photo to be rendered
     * @return a ResponseEntity of Object (that image's bytes)
     */
    @GetMapping(value = "/renderThumbnail/{token}", produces = "image/jpeg")
    public ResponseEntity<Object> renderThumbnailImage(@PathVariable String token, Integer photoId) throws IOException {
        Integer userId = verify.verifyUserByToken(token);
        Photo photo = service.getById(photoId);
        String UUID = userService.getById(userId).getUuid();
        if (userId != null && photo != null && userId.equals(photo.getUserId()))
        {
            File file = new File(uploadFolder + System.getProperty("file.separator") + UUID + System.getProperty("file.separator") + photo.getPhotoUuid()
                    + "_thumbnail" + "." + photo.getFormat());
            if (!file.exists())
            {
                file = new File(uploadFolder + System.getProperty("file.separator") + UUID + System.getProperty("file.separator") + photo.getPhotoUuid()
                        + "." + photo.getFormat());
            }
            FileInputStream inputStream = new FileInputStream(file);
            InputStreamSource inputStreamSource = new InputStreamResource(inputStream);
            return new ResponseEntity<>(inputStreamSource, new HttpHeaders(), HttpStatus.OK);
        }
        return null;
    }

    /**
     * Function to render a photo using the photo's individual token, the photo's path (explain below). Only render when
     * "visibility" of that photo is 1.
     *
     * @param token the token that belongs to the photo trying to access to.
     * @param path the path is constructed by 3 parts: <user's UUID>/<photo's UUID>.<photo's extension>.
     *             for example: 11111111-493b-47a5-bca5-4fab21ff1111/22222222-405d-49e1-1111-797abc802602.png
     *             is trying to access user with UUID (1111...)'s photo which with UUID (2222...) and this photo
     *             is a png.
     * @return a ResponseEntity of Object (that image's bytes)
     */
    @GetMapping(value = "/renderToken", produces = MediaType.IMAGE_JPEG_VALUE)
    public ResponseEntity<Object> renderImageToken(String path, String token) throws IOException {
        String photoUUID = path.split("/")[1];
        String uuid = photoUUID.substring(0, photoUUID.lastIndexOf("."));
        QueryWrapper<Photo> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("photo_uuid", uuid);
        Photo photo = service.getOne(queryWrapper);
        if (photo == null)
            return null;
        if (photo.getVisibility() == 0)
            return null;
        else if (photo.getVisibility() == 1) {
            if (token != null && token.equals(photo.getToken()))
            {
                File file = new File(uploadFolder + System.getProperty("file.separator") + path);
                FileInputStream inputStream = new FileInputStream(file);
                InputStreamSource inputStreamSource = new InputStreamResource(inputStream);
                return new ResponseEntity<>(inputStreamSource, new HttpHeaders(), HttpStatus.OK);
            }
        } else if (photo.getVisibility() == 2) {
            File file = new File(uploadFolder + System.getProperty("file.separator") + path);
            FileInputStream inputStream = new FileInputStream(file);
            InputStreamSource inputStreamSource = new InputStreamResource(inputStream);
            return new ResponseEntity<>(inputStreamSource, new HttpHeaders(), HttpStatus.OK);
        }
        return null;
    }
    /**
     * Function to render a thumbnail of a photo using the photo's individual token, the photo's path (explain below). Only render when
     * "visibility" of that photo is 1.
     *
     * @param token the token that belongs to the photo trying to access to.
     * @param path the path is constructed by 3 parts: <user's UUID>/<photo's UUID>.<photo's extension>.
     *             for example: 11111111-493b-47a5-bca5-4fab21ff1111/22222222-405d-49e1-1111-797abc802602_thumbnail.png
     *             is trying to access user with UUID (1111...)'s photo which with UUID (2222...) and this photo
     *             is a png. PLEASE NOT: _thumbnail must be included to indicate that you are trying to access a thumbnail
     * @return a ResponseEntity of Object (that image's bytes)
     */
    @GetMapping(value = "/renderToken/Thumbnail", produces = MediaType.IMAGE_JPEG_VALUE)
    public ResponseEntity<Object> renderImageTokenThumbnail(String path, String token) throws IOException {
        String photoUUID = path.split("/")[1];
        String uuid = photoUUID.substring(0, photoUUID.lastIndexOf("_"));
        QueryWrapper<Photo> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("photo_uuid", uuid);
        Photo photo = service.getOne(queryWrapper);
        if (photo == null)
            return null;
        if (photo.getVisibility() == 0)
            return null;
        else if (photo.getVisibility() == 1) {
            if (token != null && token.equals(photo.getToken()))
            {
                File file = new File(uploadFolder + System.getProperty("file.separator") + path);
                FileInputStream inputStream = new FileInputStream(file);
                InputStreamSource inputStreamSource = new InputStreamResource(inputStream);
                return new ResponseEntity<>(inputStreamSource, new HttpHeaders(), HttpStatus.OK);
            }
        } else if (photo.getVisibility() == 2) {
            File file = new File(uploadFolder + System.getProperty("file.separator") + path);
            FileInputStream inputStream = new FileInputStream(file);
            InputStreamSource inputStreamSource = new InputStreamResource(inputStream);
            return new ResponseEntity<>(inputStreamSource, new HttpHeaders(), HttpStatus.OK);
        }
        return null;
    }

    /**
     * Function to render a video using the owner's login token. Only render when
     * "visibility" of that photo is 1.
     *
     * @param token the token of the owner user of that video
     * @param photoId the video's photoId
     * @param response the servlet response
     * @return an array of bytes of that video
     */
    @GetMapping(value = "/renderV/{token}", produces = "video/mp4")
    public byte[] renderVideo(@PathVariable String token, Integer photoId, HttpServletResponse response) throws IOException {
        Integer userId = verify.verifyUserByToken(token);
        Photo photo = service.getById(photoId);
        String UUID = userService.getById(userId).getUuid();
        if (userId != null && userId.equals(photo.getUserId()))
        {
            File file = new File(uploadFolder + System.getProperty("file.separator") + UUID + System.getProperty("file.separator") + photo.getPhotoUuid()
                    + "." + photo.getFormat());
            FileInputStream inputStream = new FileInputStream(file);
            byte[] bytes = new byte[inputStream.available()];
            inputStream.read(bytes, 0, inputStream.available());
            return bytes;
        }
        return null;
    }

    /**
     * Function to render a video using the video (photo)'s individual token, the video's path (explain below). Only render when
     * "visibility" of that video (photo) is 1.
     *
     * @param token the token of the owner user of that video
     * @param path the path is constructed by 3 parts: <user's UUID>/<photo's UUID>.<video's extension>.
     *             for example: 11111111-493b-47a5-bca5-4fab21ff1111/22222222-405d-49e1-1111-797abc802602.mp4
     *             is trying to access user with UUID (1111...)'s photo which with UUID (2222...) and this video
     *             is a mp4.
     * @return an array of bytes of that video
     */
    @GetMapping(value = "/renderVToken", produces = "video/mp4")
    public byte[] renderVideoToken(String path, String token, HttpServletResponse response) throws IOException {
        String photoUUID = path.split("/")[1];
        String uuid = photoUUID.substring(0, photoUUID.lastIndexOf("."));
        QueryWrapper<Photo> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("photo_uuid", uuid);
        Photo photo = service.getOne(queryWrapper);

        if (photo == null)
            return null;
        if (photo.getVisibility() == 0)
            return null;
        else if (photo.getVisibility() == 1) {
            if (token != null && token.equals(photo.getToken()))
            {
                File file = new File(uploadFolder + System.getProperty("file.separator") + path);
                response.setContentType("application/octet-stream");
                response.setHeader("Accept-Ranges", "bytes");
                response.setHeader("Content-Length", String.valueOf(file.length()));
                response.setHeader("Content-Range", "bytes 0-" + (file.length() - 1) + "/" + file.length());
                FileInputStream inputStream = new FileInputStream(file);
                byte[] bytes = new byte[inputStream.available()];
                inputStream.read(bytes, 0, inputStream.available());
                return bytes;
            }
        } else if (photo.getVisibility() == 2) {
            File file = new File(uploadFolder + System.getProperty("file.separator") + path);
            response.setContentType("application/octet-stream");
            response.setHeader("Accept-Ranges", "bytes");
            response.setHeader("Content-Length", String.valueOf(file.length()));
            response.setHeader("Content-Range", "bytes 0-" + (file.length() - 1) + "/" + file.length());
            FileInputStream inputStream = new FileInputStream(file);
            byte[] bytes = new byte[inputStream.available()];
            inputStream.read(bytes, 0, inputStream.available());
            return bytes;
        }
        return null;
    }

    /**
     * Function to download a photo (or video) using the photoId and the user's access token (given after login)
     *
     * @param response a HttpServletResponse object for the download to work
     * @param token the token of the owner user of that photo
     * @param photoId the ID of the photo (or video) to be downloaded
     */
    @GetMapping(value = "/download/{token}")
    public void download(@PathVariable String token, Integer photoId, HttpServletResponse response) {
        Integer userId = verify.verifyUserByToken(token);
        Photo photo = service.getById(photoId);
        String UUID = userService.getById(userId).getUuid();
        if (userId != null && userId.equals(photo.getUserId()))
        {
            File file = new File(uploadFolder + System.getProperty("file.separator") + UUID + System.getProperty("file.separator") + photo.getPhotoUuid()
                    + "." + photo.getFormat());
            byte[] bytes = new byte[1024];
            BufferedInputStream bufferedInputStream = null;
            OutputStream outputStream = null;
            FileInputStream fileInputStream = null;
            try {
                fileInputStream = new FileInputStream(file);
                bufferedInputStream = new BufferedInputStream(fileInputStream);

                response.setContentType(MediaType.APPLICATION_OCTET_STREAM.toString());
                response.addHeader("Content-Disposition", "attachment;fileName=" + URLEncoder.encode(file.getName(), "UTF-8"));
                outputStream = response.getOutputStream();
                int length;
                while ((length = bufferedInputStream.read(bytes)) != -1) {
                    outputStream.write(bytes, 0, length);
                }
                outputStream.flush();
            } catch (Exception e) {
                e.getLocalizedMessage();
            } finally {
                try {
                    if (bufferedInputStream != null) {
                        bufferedInputStream.close();
                    }

                    if (outputStream != null) {
                        outputStream.close();
                    }

                    if (fileInputStream != null) {
                        fileInputStream.close();
                    }
                } catch (IOException e) {
                    e.getLocalizedMessage();
                }
            }
        }
    }

    /**
     * Function to get a photo's out-site render path (render using that photo's token and path)
     *
     * @param request a HttpServletRequest to verify the user's token
     * @param photoId the ID of the photo (or video) to get the path
     * @return a result of string which is the path of that photo (or video)
     * example:
     * {
     *     "t": "127.0.0.1:8084/photo/renderVToken?path=11111111-493b-47a5-bca5-4fab21ffaa46/6f0b2f16-1111-47b6-9a69-161e35ed77d6.mp4&token=11111111-ecb7-4f64-1111-1111158da4e0",
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getPath")
    public Result<String> getImagePath(HttpServletRequest request, Integer photoId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        String UUID = userService.getById(userId).getUuid();
        QueryWrapper<Photo> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("photo_id", photoId);
        queryWrapper.eq("user_id", userId);
        Photo photo = service.getOne(queryWrapper);
        if (photo != null)
        {
            String format = photo.getFormat().toLowerCase();
            if (format.equals("jpg") || format.equals("jpeg") || format.equals("gif") || format.equals("png")
                    || format.equals("webp")|| format.equals("ico") || format.equals("bmp")|| format.equals("tif")
                    || format.equals("svg")|| format.equals("psd") || format.equals("raw")|| format.equals("pcd"))
            {
                String path = serverPath + "/photo/renderToken?path=" + UUID + "/" + photo.getPhotoUuid() + "." + photo.getFormat()
                        + "&token=" + photo.getToken();
                return new Result<>(path, 200);
            }else if (format.equals("mp4") || format.equals("avi") || format.equals("wmv") || format.equals("mpeg")
            || format.equals("m4v") || format.equals("mov") || format.equals("flv") || format.equals("asf")
                    || format.equals("f4v") || format.equals("rmvb") || format.equals("vob") || format.equals("rm"))
            {
                String path = serverPath + "/photo/renderVToken?path=" + UUID + "/" + photo.getPhotoUuid() + "." + photo.getFormat()
                        + "&token=" + photo.getToken();
                return new Result<>(path, 200);
            }else {
                return new Result<>(null, 402);
            }

        }else
            return new Result<>(null, 403);
    }

    /**
     * Function to get a photo's out-site rendering QR code
     *
     * @param request a HttpServletRequest to verify the user's token
     * @param photoId the ID of the photo (or video) to get the path
     * @return a result of bytes which can be constructed to a QR code image
     */
    @GetMapping(value = "/getQR", produces = MediaType.IMAGE_JPEG_VALUE)
    public byte[] getImageQR(HttpServletRequest request, Integer photoId)
    {
        String URL = getImagePath(request, photoId).getT();
        if (URL != null)
        {
            QRCode qrCode;
            if (URL.contains("http") || URL.contains("https"))
            {
                qrCode = QRCode.from(URL).withSize(250, 250);
            }else {
                qrCode = QRCode.from("http://" + URL).withSize(250, 250);
            }
            return qrCode.stream().toByteArray();
        }
        return new byte[]{};
    }

    /**
     * Function to get a photo's out-site rendering QR code
     *
     * @param token the user's token
     * @param photoId the ID of the photo (or video) to get the path
     * @return a result of bytes which can be constructed to a QR code image
     */
    @GetMapping(value = "/getQR2", produces = MediaType.IMAGE_JPEG_VALUE)
    public byte[] getImageQR2(String token, Integer photoId)
    {
        String QRPath = "";

        Integer userId = verify.verifyUserByToken(token);
        if (userId < 0)
            return new byte[]{};
        String UUID = userService.getById(userId).getUuid();
        QueryWrapper<Photo> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("photo_id", photoId);
        queryWrapper.eq("user_id", userId);
        Photo photo = service.getOne(queryWrapper);
        if (photo != null)
        {
            String format = photo.getFormat().toLowerCase();
            if (format.equals("jpg") || format.equals("jpeg") || format.equals("gif") || format.equals("png")
                    || format.equals("webp")|| format.equals("ico") || format.equals("bmp")|| format.equals("tif")
                    || format.equals("svg")|| format.equals("psd") || format.equals("raw")|| format.equals("pcd"))
            {
                String path = serverPath + "/photo/renderToken?path=" + UUID + "/" + photo.getPhotoUuid() + "." + photo.getFormat()
                        + "&token=" + photo.getToken();
                QRPath = path;
            }else if (format.equals("mp4") || format.equals("avi") || format.equals("wmv") || format.equals("mpeg")
                    || format.equals("m4v") || format.equals("mov") || format.equals("flv") || format.equals("asf")
                    || format.equals("f4v") || format.equals("rmvb") || format.equals("vob") || format.equals("rm"))
            {
                String path = serverPath + "/photo/renderVToken?path=" + UUID + "/" + photo.getPhotoUuid() + "." + photo.getFormat()
                        + "&token=" + photo.getToken();
                QRPath = path;
            }else {
                return new byte[]{};
            }

        }else
            return new byte[]{};


        String URL = QRPath;
        QRCode qrCode;
        if (URL.contains("http") || URL.contains("https"))
        {
            qrCode = QRCode.from(URL).withSize(250, 250);
        }else {
            qrCode = QRCode.from("http://" + URL).withSize(250, 250);
        }
        return qrCode.stream().toByteArray();
    }

    /**
     * Function to search photo by conditions
     *
     * @param request a HttpServletRequest to verify the user's token
     * @param option a string that could either be "photoName, tag, gallery, or namespace"
     * @param query a string that is the keyword you want to query
     * @return a Result of List of Photos which are related to the query
     */
    @GetMapping(value = "/searchPhoto")
    public Result<List<Photo>> searchPhoto(HttpServletRequest request, String option, String query)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        List<Photo> retList = service.searchPhoto(userId, option, query);
        if (retList == null || retList.size() == 0)
            return new Result<>(retList, 404);
        return new Result<>(retList, 200);
    }

    /**
     * Function to get Photo's Exif information
     *
     * @param request a HttpServletRequest to verify the user's token
     * @param photoId the photoId of the photo you want to get the information on
     * @return a Result of LinkedHashMao (dictionary) of Exif info
     */
    @GetMapping(value = "/getPhotoExif")
    public Result<LinkedHashMap<String, String>> getPhotoExif(HttpServletRequest request, Integer photoId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        String UUID = userService.getById(userId).getUuid();
        LinkedHashMap<String, String> res = service.getPhotoExif(userId, photoId, UUID);
        if (res == null)
            return new Result<>(null, 404);
        return new Result<>(res, 200);

    }
}

