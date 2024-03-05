package org.chengbing.controller;


import org.chengbing.service.IPhotoExtraService;
import org.chengbing.service.IUserService;
import org.chengbing.util.Result;
import org.chengbing.util.UserIdentity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;

import org.springframework.web.bind.annotation.RestController;

import javax.annotation.Resource;
import javax.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.List;

/**
 * <p>
 *  前端控制器
 * </p>
 *
 * @author HaroldCI
 * @since 2024-03-04
 */
@RestController
@RequestMapping("/photo-extra")
public class PhotoExtraController {

    @Resource
    IPhotoExtraService service;

    @Resource
    UserIdentity verify;

    @Resource
    IUserService userService;

    /**
     * Function to get Photo's information and extra information for global map usage
     *
     * @param request a HttpServletRequest to verify the user's token
     * @return a Result of List of LinkedHashMao (dictionary) of photo info
     */
    @GetMapping(value = "/getGlobalMapPhoto")
    public Result<List<LinkedHashMap<String, Object>>> queryGlobalPhoto(HttpServletRequest request)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        List<LinkedHashMap<String, Object>> res = service.queryGlobalPhoto(userId);
        if (res == null)
            return new Result<>(null, 404);
        return new Result<>(res, 200);

    }

    /**
     * Function to get Photo's information and extra information for global map usage
     *
     * @param request a HttpServletRequest to verify the user's token
     * @param gaId the gallery's ID
     * @return a Result of List of LinkedHashMao (dictionary) of photo info
     */
    @GetMapping(value = "/getGlobalMapPhotoByGallery")
    public Result<List<LinkedHashMap<String, Object>>> getGlobalMapPhotoByGallery(HttpServletRequest request, Integer gaId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        List<LinkedHashMap<String, Object>> res = service.queryGalleryPhoto(userId, gaId);
        if (res == null)
            return new Result<>(null, 404);
        return new Result<>(res, 200);

    }
}

