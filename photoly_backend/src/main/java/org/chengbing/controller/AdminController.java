package org.chengbing.controller;

import io.swagger.v3.oas.annotations.links.Link;
import org.chengbing.entity.User;
import org.chengbing.service.IAdminService;
import org.chengbing.util.Result;
import org.chengbing.util.ResultPage;
import org.chengbing.util.UserIdentity;
import org.springframework.beans.BeansException;
import org.springframework.context.ApplicationContext;
import org.springframework.context.ApplicationContextAware;
import org.springframework.web.bind.annotation.*;

import javax.annotation.Resource;
import javax.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.List;

***REMOVED***
 * @Author: Harold澂冰
 * @Date: 2022/7/15 15:27
***REMOVED***
@RestController
@RequestMapping("/admin")
public class AdminController {

    @Resource
    UserIdentity verify;

    @Resource
    IAdminService service;


    @GetMapping("/isAdmin")
    public Result<Boolean> isAdmin(HttpServletRequest request)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(Boolean.FALSE, 403);
        boolean res = service.isAdmin(userId);
        return new Result<>(res, 200);
***REMOVED***

    @GetMapping("/pingMySQL")
    public Result<Integer> pingMySQL(HttpServletRequest request)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.pingMySQL(userId);
        return res == 1 ? new Result<>(res, 200) : new Result<>(res, 400);
***REMOVED***

    @GetMapping("/pingRedis")
    public Result<Integer> pingRedis(HttpServletRequest request)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.pingRedis(userId);
        return res == 1 ? new Result<>(res, 200) : new Result<>(res, 400);
***REMOVED***

    @GetMapping("/getAddress")
    public Result<LinkedHashMap<String, String>> getAddress(HttpServletRequest request)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        LinkedHashMap<String, String> map = service.getAddress(userId);
        return map == null ? new Result<>(null, 400) : new Result<>(map, 200);
***REMOVED***

    @GetMapping("/getSettings")
    public Result<LinkedHashMap<String, String>> getSettings(HttpServletRequest request)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        LinkedHashMap<String, String> map = service.getSettings(userId);
        return map == null ? new Result<>(null, 400) : new Result<>(map, 200);
***REMOVED***

    @PostMapping("/stop")
    public Result<Integer> stopServer(HttpServletRequest request)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        return new Result<>(service.stopServer(userId), 591023);
***REMOVED***

    @PostMapping("/setSignUpPermission")
    public Result<Integer> setSignUpPermission(HttpServletRequest request, Integer permission)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.setSignUpPermission(userId, permission);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
***REMOVED***

    @PostMapping("/setSSafeUUIDMode")
    public Result<Integer> setSSafeUUIDMode(HttpServletRequest request, Integer mode)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.setSSafeUUIDMode(userId, mode);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
***REMOVED***

    @PostMapping("/setTokenDuration")
    public Result<Integer> setTokenDuration(HttpServletRequest request, Integer days)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        if (days < 0 || days > 365)
            return new Result<>(-1, 403);
        int res = service.setTokenDuration(userId, days);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
***REMOVED***

    @GetMapping("/getUserPage")
    public ResultPage<List<User>> getUserPage(HttpServletRequest request, Integer page, Integer rowsPerPage)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new ResultPage<>(null, 0, 403);
        return service.getUserPage(userId, page, rowsPerPage);
***REMOVED***

    @GetMapping("/searchUserPage")
    public ResultPage<List<User>> searchUserPage(HttpServletRequest request, String search, Integer page, Integer rowsPerPage)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new ResultPage<>(null, 0, 403);
        return service.searchUserPage(userId, search, page, rowsPerPage);
***REMOVED***

    @PostMapping("/addUser")
    public Result<Integer> addUser(HttpServletRequest request, @RequestBody User user)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.addUser(userId, user);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
***REMOVED***

    @PostMapping("/deleteUser")
    public Result<Integer> deleteUser(HttpServletRequest request, Integer userId)
***REMOVED***
        Integer id = verify.verifyUser(request);
        if (id < 0)
            return new Result<>(-2, 403);
        int res = service.deleteUser(id, userId);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
***REMOVED***

    @PostMapping("/resetUserPassword")
    public Result<Integer> resetUserPassword(HttpServletRequest request, @RequestBody User user)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.resetUserPassword(userId, user);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
***REMOVED***

    @PostMapping("/resetUserEmail")
    public Result<Integer> resetUserEmail(HttpServletRequest request, @RequestBody User user)
***REMOVED***
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.resetUserEmail(userId, user);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
***REMOVED***
***REMOVED***
