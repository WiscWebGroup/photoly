package org.chengbing.controller;


import org.chengbing.entity.Tag;
import org.chengbing.service.ITagService;
import org.chengbing.util.Result;
import org.chengbing.util.UserIdentity;
import org.springframework.web.bind.annotation.*;

import javax.annotation.Resource;
import javax.servlet.http.HttpServletRequest;
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
@RequestMapping("/tag")
public class TagController {
    @Resource
    ITagService service;

    @Resource
    UserIdentity verify;

    @GetMapping("/getAll")
    public Result<List<Tag>> queryTag(HttpServletRequest request)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryTag(userId), 200);
    }

    @PostMapping("/delete")
    public Result<Integer> deleteTag(HttpServletRequest request, Integer tagId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.deleteTag(userId, tagId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @PostMapping("/update")
    public Result<Integer> updateTag(HttpServletRequest request,@RequestBody Tag tag)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updateTag(userId, tag);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @PostMapping("/insert")
    public Result<Integer> insertTag(HttpServletRequest request, @RequestBody Tag tag)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-1, 403);
        int change = service.insertTag(userId, tag);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }
}

