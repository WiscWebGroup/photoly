package org.chengbing.controller;


import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
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

    @Value("file.uploadFolder")
    String uploadFolder;

    @Value("server.path")
    String serverPath;

    @PostMapping("/insert")
    public Result<Integer> insertPhoto(HttpServletRequest request, MultipartFile file, @RequestBody Photo photo)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.insertPhoto(userId, file, photo);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @PostMapping("/inserts")
    public Result<Integer> insertPhotos(HttpServletRequest request, MultipartFile[] files, @RequestBody List<Photo> photo)
    {
        if (files.length != photo.size())
            return null;
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int total = photo.size();
        int change = 0;
        for (int i = 0; i < total; i ++)
            change += service.insertPhoto(userId, files[i], photo.get(i));
        return change >= 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @PostMapping("/delete")
    public Result<Integer> deletePhoto(HttpServletRequest request, Integer photoId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.deletePhoto(userId, photoId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @PostMapping("/update")
    public Result<Integer> updatePhotoNameAndVisibility(HttpServletRequest request, @RequestBody Photo photo)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updatePhotoNameAndVisibility(userId, photo);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @PostMapping("/getByNamespace")
    public Result<List<Photo>> queryPhotoByNamespace(HttpServletRequest request, Integer nsId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryPhotoByNamespace(userId, nsId), 200);
    }

    @PostMapping("/getByTags")
    public Result<List<Photo>> queryPhotoByTags(HttpServletRequest request, List<Integer> tagIds)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryPhotoByTags(userId, tagIds), 200);
    }

    @PostMapping("/getByGallery")
    public Result<List<Photo>> queryPhotoByGallery(HttpServletRequest request, Integer gaId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryPhotoByGallery(userId, gaId), 200);
    }

    @PostMapping("/changeNamespace")
    public Result<Integer> changeNamespace(HttpServletRequest request, Integer photoId, Integer nsId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.changeNamespace(userId, photoId, nsId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @PostMapping("/addTags")
    public Result<Integer> addTags(HttpServletRequest request, Integer photoId, List<Integer> tagIds)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.addTags(userId, photoId, tagIds);
        return change >= 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @PostMapping("/deleteTag")
    public Result<Integer> deleteTag(HttpServletRequest request, Integer photoId, Integer tagId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.deleteTag(userId, photoId, tagId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @PostMapping("/addToGallery")
    public Result<Integer> addToGallery(HttpServletRequest request, Integer photoId, Integer gaId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.addToGallery(userId, photoId, gaId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @PostMapping("/deleteFromGallery")
    public Result<Integer> deleteFromGallery(HttpServletRequest request, Integer photoId, Integer gaId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.deleteFromGallery(userId, photoId, gaId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @GetMapping(value = "/render", produces = MediaType.IMAGE_JPEG_VALUE)
    public byte[] renderImage(HttpServletRequest request, Integer photoId) throws IOException {
        Integer userId = verify.verifyUser(request);
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

    @GetMapping(value = "/renderToken", produces = MediaType.IMAGE_JPEG_VALUE)
    public byte[] renderImageToken(String path, String token) throws IOException {
        QueryWrapper<Photo> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("token", token);
        Photo photo = service.getOne(queryWrapper);
        Integer userId = photo.getUserId();
        if (photo.getVisibility() == 0)
            return null;
        else if (photo.getVisibility() == 1) {
            if (token.equals(photo.getToken()))
            {
                File file = new File(uploadFolder + System.getProperty("file.separator") + path);
                FileInputStream inputStream = new FileInputStream(file);
                byte[] bytes = new byte[inputStream.available()];
                inputStream.read(bytes, 0, inputStream.available());
                return bytes;
            }
        } else if (photo.getVisibility() == 2) {
            File file = new File(uploadFolder + System.getProperty("file.separator") + path);
            FileInputStream inputStream = new FileInputStream(file);
            byte[] bytes = new byte[inputStream.available()];
            inputStream.read(bytes, 0, inputStream.available());
            return bytes;
        }
        return null;
    }

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
        String path = serverPath + "/photo/renderToken?path=" + UUID + "/" + photo.getPhotoUuid() + "." + photo.getFormat()
                 + "&token=" + photo.getToken();
        return new Result<>(path, 200);
    }
}

