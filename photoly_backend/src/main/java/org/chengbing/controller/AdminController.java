package org.chengbing.controller;

import org.chengbing.entity.User;
import org.chengbing.util.Result;
import org.springframework.web.bind.annotation.*;

import javax.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.List;

/**
 * @Author: Harold澂冰
 * @Date: 2022/7/15 15:27
 */
@RestController
@RequestMapping("/admin")
public class AdminController {

    @GetMapping("/isAdmin")
    public Result<Boolean> isAdmin(HttpServletRequest request)
    {
        return null;
    }

    @GetMapping("/pingMySQL")
    public Result<Integer> pingMySQL(HttpServletRequest request)
    {
        return null;
    }

    @GetMapping("/pingRedis")
    public Result<Integer> pingRedis(HttpServletRequest request)
    {
        return null;
    }

    @GetMapping("/getAddress")
    public Result<LinkedHashMap<String, String>> getAddress(HttpServletRequest request)
    {
        return null;
    }

    @GetMapping("/stop")
    public Result<Integer> stopServer(HttpServletRequest request)
    {
        return null;
    }

    @PostMapping("/setSignUpPermission")
    public Result<Integer> setSignUpPermission(HttpServletRequest request, Integer permission)
    {
        return null;
    }

    @PostMapping("/setSSafeUUIDMode")
    public Result<Integer> setSSafeUUIDMode(HttpServletRequest request, Integer mode)
    {
        return null;
    }

    @PostMapping("/setTokenDuration")
    public Result<Integer> setTokenDuration(HttpServletRequest request, Integer days)
    {
        return null;
    }

    @GetMapping("/getUserPage")
    public Result<List<User>> getUserPage(HttpServletRequest request, Integer page, Integer rowsPerPage)
    {
        return null;
    }

    @GetMapping("/searchUserPage")
    public Result<List<User>> searchUserPage(HttpServletRequest request, String search, Integer page, Integer rowsPerPage)
    {
        return null;
    }

    @PostMapping("/addUser")
    public Result<List<User>> searchUserPage(HttpServletRequest request, @RequestBody User user)
    {
        return null;
    }

    @PostMapping("/deleteUser")
    public Result<List<User>> deleteUser(HttpServletRequest request, @RequestBody Integer userId)
    {
        return null;
    }

    @PostMapping("/resetUserPassword")
    public Result<Integer> resetUserPassword(HttpServletRequest request, @RequestBody User user)
    {
        return null;
    }

    @PostMapping("/resetUserEmail")
    public Result<Integer> resetUserEmail(HttpServletRequest request, @RequestBody User user)
    {
        return null;
    }
}
