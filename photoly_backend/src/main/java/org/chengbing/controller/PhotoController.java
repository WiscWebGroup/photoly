package org.chengbing.controller;


import com.alibaba.fastjson.JSONObject;
import com.alibaba.fastjson.TypeReference;
import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.fasterxml.jackson.databind.util.JSONPObject;
import org.chengbing.entity.Photo;
import org.chengbing.service.IPhotoService;
import org.chengbing.service.IUserService;
import org.chengbing.util.Result;
import org.chengbing.util.UserIdentity;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.*;

import org.springframework.web.multipart.MultipartFile;

import javax.annotation.Resource;
import javax.servlet.http.HttpServletRequest;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileNotFoundException;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;

***REMOVED***
 * <p>
 *  前端控制器
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
@RestController
@RequestMapping("/photo")
public class PhotoController {
    @Resource
    IPhotoService service;

    @Resource
    UserIdentity verify;

    @Resource
    IUserService userService;

    @Value("${file.uploadFolder***REMOVED***")
    String uploadFolder;

    @Value("${server.path***REMOVED***")
    String serverPath;

    @PostMapping("/insert")
    public Result<Integer> insertPhoto(HttpServletRequest request, MultipartFile file, @RequestParam String photo)
***REMOVED***
        Photo photo1 = JSONObject.parseObject(photo, Photo.class);
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.insertPhoto(userId, file, photo1);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
***REMOVED***

    @PostMapping("/inserts")
    public Result<Integer> insertPhotos(HttpServletRequest request, MultipartFile[] files, @RequestParam String photosStr)
***REMOVED***
        List<Photo> photos = JSONObject.parseObject(photosStr, new TypeReference<ArrayList<Photo>>(){***REMOVED***);
        if (files.length != photos.size())
    ***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.insertPhotos(userId, files, photos);
        return change >= 1 ? new Result<>(change, 200) : new Result<>(change, 400);
***REMOVED***

    @PostMapping("/delete")
    public Result<Integer> deletePhoto(HttpServletRequest request, Integer photoId)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.deletePhoto(userId, photoId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
***REMOVED***

    @PostMapping("/update")
    public Result<Integer> updatePhotoNameAndVisibility(HttpServletRequest request, @RequestBody Photo photo)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updatePhotoNameAndVisibility(userId, photo);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
***REMOVED***

    @GetMapping("/getByNamespace")
    public Result<List<Photo>> queryPhotoByNamespace(HttpServletRequest request, Integer nsId)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryPhotoByNamespace(userId, nsId), 200);
***REMOVED***

    @GetMapping("/getByTags")
    public Result<List<Photo>> queryPhotoByTags(HttpServletRequest request, @RequestParam List<Integer> tagIds)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryPhotoByTags(userId, tagIds), 200);
***REMOVED***

    @GetMapping("/getByGallery")
    public Result<List<Photo>> queryPhotoByGallery(HttpServletRequest request, Integer gaId)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryPhotoByGallery(userId, gaId), 200);
***REMOVED***

    @PostMapping("/changeNamespace")
    public Result<Integer> changeNamespace(HttpServletRequest request, Integer photoId, Integer nsId)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.changeNamespace(userId, photoId, nsId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
***REMOVED***

    @PostMapping("/addTags")
    public Result<Integer> addTags(HttpServletRequest request, Integer photoId, @RequestParam List<Integer> tagIds)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.addTags(userId, photoId, tagIds);
        return change >= 1 ? new Result<>(change, 200) : new Result<>(change, 400);
***REMOVED***

    @PostMapping("/deleteTag")
    public Result<Integer> deleteTag(HttpServletRequest request, Integer photoId, Integer tagId)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.deleteTag(userId, photoId, tagId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
***REMOVED***

    @PostMapping("/addToGallery")
    public Result<Integer> addToGallery(HttpServletRequest request, Integer photoId, Integer gaId)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.addToGallery(userId, photoId, gaId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
***REMOVED***

    @PostMapping("/deleteFromGallery")
    public Result<Integer> deleteFromGallery(HttpServletRequest request, Integer photoId, Integer gaId)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.deleteFromGallery(userId, photoId, gaId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
***REMOVED***

    @GetMapping(value = "/render", produces = MediaType.IMAGE_JPEG_VALUE)
    public byte[] renderImage(HttpServletRequest request, Integer photoId) throws IOException {
        Integer userId = verify.verifyUser(request);
        Photo photo = service.getById(photoId);
        String UUID = userService.getById(userId).getUuid();
        if (userId != null && userId.equals(photo.getUserId()))
    ***REMOVED***
            File file = new File(uploadFolder + System.getProperty("file.separator") + UUID + System.getProperty("file.separator") + photo.getPhotoUuid()
             + "." + photo.getFormat());
            FileInputStream inputStream = new FileInputStream(file);
            byte[] bytes = new byte[inputStream.available()];
            inputStream.read(bytes, 0, inputStream.available());
            return bytes;
***REMOVED***
***REMOVED***
***REMOVED***

    @GetMapping(value = "/renderToken", produces = MediaType.IMAGE_JPEG_VALUE)
    public byte[] renderImageToken(String path, String token) throws IOException {
        String photoUUID = path.split("/")[1];
        String uuid = photoUUID.substring(0, photoUUID.lastIndexOf("."));
        QueryWrapper<Photo> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("photo_uuid", uuid);
        Photo photo = service.getOne(queryWrapper);
        if (photo == null)
    ***REMOVED***
        if (photo.getVisibility() == 0)
    ***REMOVED***
        else if (photo.getVisibility() == 1) {
            if (token != null && token.equals(photo.getToken()))
        ***REMOVED***
                File file = new File(uploadFolder + System.getProperty("file.separator") + path);
                FileInputStream inputStream = new FileInputStream(file);
                byte[] bytes = new byte[inputStream.available()];
                inputStream.read(bytes, 0, inputStream.available());
                return bytes;
    ***REMOVED***
***REMOVED*** else if (photo.getVisibility() == 2) {
            File file = new File(uploadFolder + System.getProperty("file.separator") + path);
            FileInputStream inputStream = new FileInputStream(file);
            byte[] bytes = new byte[inputStream.available()];
            inputStream.read(bytes, 0, inputStream.available());
            return bytes;
***REMOVED***
***REMOVED***
***REMOVED***

    @GetMapping("/getPath")
    public Result<String> getImagePath(HttpServletRequest request, Integer photoId)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        String UUID = userService.getById(userId).getUuid();
        QueryWrapper<Photo> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("photo_id", photoId);
        queryWrapper.eq("user_id", userId);
        Photo photo = service.getOne(queryWrapper);
        if (photo != null)
    ***REMOVED***
            String path = serverPath + "/photo/renderToken?path=" + UUID + "/" + photo.getPhotoUuid() + "." + photo.getFormat()
                     + "&token=" + photo.getToken();
            return new Result<>(path, 200);
***REMOVED***else
            return new Result<>(null, 403);
***REMOVED***
***REMOVED***

