package org.chengbing.controller;


import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.metadata.IPage;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import org.chengbing.entity.Gallery;
import org.chengbing.service.IGalleryService;
import org.chengbing.util.Result;
import org.chengbing.util.ResultPage;
import org.chengbing.util.UserIdentity;
import org.springframework.data.redis.core.RedisTemplate;
import org.springframework.web.bind.annotation.*;

import javax.annotation.Resource;
import javax.servlet.http.HttpServletRequest;
import java.time.LocalDateTime;
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
@RequestMapping("/gallery")
public class GalleryController {
    @Resource
    IGalleryService service;
    @Resource
    UserIdentity verify;

    /**
     * Function to create a new gallery. The object passed in must have "gaName" and "coverColor". optional "coverId"
     * coverColor is in hex color format, for example, "#f589cd". CoverId should be in range 1-10.
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param gallery is a Gallery object with some params
     * example:
     * {
     *     "gaName": "testGallery1",
     *     "coverId": 1,
     *     "coverColor": "#f589cd"
     * }
     *
     * @return a result of boolean indicates success or not. (true, 200) means success
     * example:
     * {
     *     "t": true,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/insert")
    public Result<Boolean> insertGallery(HttpServletRequest request, @RequestBody Gallery gallery)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(false, 403);
        if (gallery.getGaName() == null || gallery.getGaName().equals(""))
            return new Result<>(false, 400);
        gallery.setGaId(null);
        gallery.setUserId(userId);
        gallery.setCreateDate(LocalDateTime.now());
        boolean res = service.save(gallery);
        if (!res)
            return new Result<>(false, 401);
        return new Result<>(true, 200);
    }

    /**
     * Function to get all galleries created by this user
     * @param request an HttpServletRequest which pass in the current user's access token
     * @return a result of list of gallery objects
     * example:
     * {
     *     "t": [
     *         {
     *             "gaId": 11,
     *             "gaName": "My Favorite",
     *             "userId": 321,
     *             "createDate": "2023-12-02T16:34:27",
     *             "coverId": 1,
     *             "coverColor": "#FFFFFF"
     *         },
     *         {
     *             "gaId": 12,
     *             "gaName": "testGallery1",
     *             "userId": 321,
     *             "createDate": "2023-05-12T14:11:12",
     *             "coverId": 1,
     *             "coverColor": "#f589cd"
     *         }
     *     ],...
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getAll")
    public Result<List<Gallery>> queryGallery(HttpServletRequest request)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryGallery(userId), 200);
    }

    /**
     * Function to get galleries based on pagination
     * @param request an HttpServletRequest which pass in the current user's access token
     * @return a result of list of gallery objects
     * example (with page=1 and rowsPerPage=1):
     * {
     *     "t": [
     *         {
     *             "gaId": 11,
     *             "gaName": "My Favorite",
     *             "userId": 321,
     *             "createDate": "2023-02-02T16:34:27",
     *             "coverId": null,
     *             "coverColor": "#FFFFFF"
     *         }
     *     ],
     *     "pageNum": 2,
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getPage")
    public ResultPage<List<Gallery>> queryGalleryPage(HttpServletRequest request, Integer page, Integer rowsPerPage)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new ResultPage<>(-1, 403);
        return service.queryGalleryPage(userId, page, rowsPerPage);
    }

    /**
     * Function to delete a gallery
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param gaId the ID of the gallery to be deleted
     * @return a result of integer suggesting if the operation is successful
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/delete")
    public Result<Integer> deleteGallery(HttpServletRequest request, Integer gaId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-1, 403);
        int res = service.deleteGallery(userId, gaId);
        return res == 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function to update a gallery
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param gallery a gallery object
     * example:
     * {
     *     "gaId": 952,
     *     "gaName": "renamed_gallery123",
     *     "coverId": 1,
     *     "coverColor": "#f489bd"
     * }
     * @return a result of integer suggesting if the operation is successful
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/update")
    public Result<Integer> updateGallery(HttpServletRequest request,@RequestBody Gallery gallery)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-1, 403);
        int res = service.updateGallery(userId, gallery.getGaId(), gallery);
        return res == 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }
}

