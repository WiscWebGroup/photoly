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

    @PostMapping("/insert")
    public Result<Boolean> insertGallery(HttpServletRequest request, @RequestBody Gallery gallery)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(false, 403);
        if (gallery.getGaName() == null || gallery.getGaName().equals(""))
            return new Result<>(false, 400);
        gallery.setUserId(userId);
        gallery.setCreateDate(LocalDateTime.now());
        boolean res = service.save(gallery);
        if (!res)
            return new Result<>(false, 401);
        return new Result<>(true, 200);
    }

    @GetMapping("/getAll")
    public Result<List<Gallery>> queryGallery(HttpServletRequest request)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryGallery(userId), 200);
    }

    @GetMapping("/getPage")
    public ResultPage<List<Gallery>> queryGalleryPage(HttpServletRequest request, Integer page, Integer rowsPerPage)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new ResultPage<>(-1, 403);
        return service.queryGalleryPage(userId, page, rowsPerPage);
    }

    @DeleteMapping("/delete")
    public Result<Integer> deleteGallery(HttpServletRequest request, Integer gaId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-1, 403);
        return new Result<>(service.deleteGallery(userId, gaId), 200);
    }

    @PostMapping("/update")
    public Result<Integer> updateGallery(HttpServletRequest request,@RequestBody Gallery gallery)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-1, 403);
        return new Result<>(service.updateGallery(userId, gallery.getGaId(), gallery), 200);
    }
}

