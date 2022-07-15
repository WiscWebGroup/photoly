package org.chengbing.controller;

import org.chengbing.entity.User;
import org.chengbing.util.Result;
import org.springframework.web.bind.annotation.*;

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

    @GetMapping("/isAdmin")
    public Result<Boolean> isAdmin(HttpServletRequest request)
***REMOVED***
***REMOVED***
***REMOVED***

    @GetMapping("/pingMySQL")
    public Result<Integer> pingMySQL(HttpServletRequest request)
***REMOVED***
***REMOVED***
***REMOVED***

    @GetMapping("/pingRedis")
    public Result<Integer> pingRedis(HttpServletRequest request)
***REMOVED***
***REMOVED***
***REMOVED***

    @GetMapping("/getAddress")
    public Result<LinkedHashMap<String, String>> getAddress(HttpServletRequest request)
***REMOVED***
***REMOVED***
***REMOVED***

    @GetMapping("/stop")
    public Result<Integer> stopServer(HttpServletRequest request)
***REMOVED***
***REMOVED***
***REMOVED***

    @PostMapping("/setSignUpPermission")
    public Result<Integer> setSignUpPermission(HttpServletRequest request, Integer permission)
***REMOVED***
***REMOVED***
***REMOVED***

    @PostMapping("/setSSafeUUIDMode")
    public Result<Integer> setSSafeUUIDMode(HttpServletRequest request, Integer mode)
***REMOVED***
***REMOVED***
***REMOVED***

    @PostMapping("/setTokenDuration")
    public Result<Integer> setTokenDuration(HttpServletRequest request, Integer days)
***REMOVED***
***REMOVED***
***REMOVED***

    @GetMapping("/getUserPage")
    public Result<List<User>> getUserPage(HttpServletRequest request, Integer page, Integer rowsPerPage)
***REMOVED***
***REMOVED***
***REMOVED***

    @GetMapping("/searchUserPage")
    public Result<List<User>> searchUserPage(HttpServletRequest request, String search, Integer page, Integer rowsPerPage)
***REMOVED***
***REMOVED***
***REMOVED***

    @PostMapping("/addUser")
    public Result<List<User>> searchUserPage(HttpServletRequest request, @RequestBody User user)
***REMOVED***
***REMOVED***
***REMOVED***

    @PostMapping("/deleteUser")
    public Result<List<User>> deleteUser(HttpServletRequest request, @RequestBody Integer userId)
***REMOVED***
***REMOVED***
***REMOVED***

    @PostMapping("/resetUserPassword")
    public Result<Integer> resetUserPassword(HttpServletRequest request, @RequestBody User user)
***REMOVED***
***REMOVED***
***REMOVED***

    @PostMapping("/resetUserEmail")
    public Result<Integer> resetUserEmail(HttpServletRequest request, @RequestBody User user)
***REMOVED***
***REMOVED***
***REMOVED***
***REMOVED***
