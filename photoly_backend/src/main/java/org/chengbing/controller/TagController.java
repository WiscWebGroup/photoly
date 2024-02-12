package org.chengbing.controller;


import org.chengbing.entity.Tag;
import org.chengbing.service.ITagService;
import org.chengbing.util.Result;
import org.chengbing.util.ResultPage;
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

    /**
     * Function to get all tags created by this user
     * @param request an HttpServletRequest which pass in the current user's access token
     * @return a result of list of Tag objects
     * example:
     * {
     *     "t": [
     *         {
     *             "tagId": 90,
     *             "userId": 321,
     *             "tagName": "testTag1"
     *         },
     *         {
     *             "tagId": 91,
     *             "userId": 321,
     *             "tagName": "testTag2"
     *         },...
     *     ],
     *     "msgCode": 200
     * }
     *
     */
    @GetMapping("/getAll")
    public Result<List<Tag>> queryTag(HttpServletRequest request)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        return new Result<>(service.queryTag(userId), 200);
    }

    /**
     * Function to get tags based on pagination
     * @param request an HttpServletRequest which pass in the current user's access token
     * @return a result of list of gallery objects
     * example (with page=1 and rowsPerPage=2):
     * {
     *     "t": [
     *         {
     *             "tagId": 111,
     *             "userId": 321,
     *             "tagName": "testTag1"
     *         },
     *         {
     *             "tagId": 112,
     *             "userId": 321,
     *             "tagName": "testTag2"
     *         }
     *     ],
     *     "pageNum": 2,
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getPage")
    public ResultPage<List<Tag>> queryTagPage(HttpServletRequest request, Integer page, Integer rowsPerPage)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new ResultPage<>(-1, 403);
        return service.queryTagPage(userId, page, rowsPerPage);
    }

    /**
     * Function to delete a tag
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param tagId the ID of the tag to be deleted
     * @return a result of integer suggesting if the operation is successful
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/delete")
    public Result<Integer> deleteTag(HttpServletRequest request, Integer tagId)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.deleteTag(userId, tagId);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to update a tag
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param tag a Tag object "probably with different tagName
     * example:
     * {
     *     "tagId": 782,
     *     "tagName": "testTag123"
     * }
     * @return a result of integer suggesting if the operation is successful
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/update")
    public Result<Integer> updateTag(HttpServletRequest request,@RequestBody Tag tag)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updateTag(userId, tag);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /**
     * Function to create a new tag. The object passed in must have "tagName"
     * @param request an HttpServletRequest which pass in the current user's access token
     * @param tag is a Tag object with "tagName" params
     * example:
     * {
     *     "tagName": "testTag1"
     * }
     *
     * @return a result of Integer indicates success or not. (1, 200) means success
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
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

