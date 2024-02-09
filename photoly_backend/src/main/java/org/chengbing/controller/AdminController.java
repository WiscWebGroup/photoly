package org.chengbing.controller;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import io.swagger.v3.oas.annotations.links.Link;
import org.chengbing.entity.Setting;
import org.chengbing.entity.User;
import org.chengbing.service.IAdminService;
import org.chengbing.util.Result;
import org.chengbing.util.ResultPage;
import org.chengbing.util.UserIdentity;
import org.springframework.web.bind.annotation.*;

import javax.annotation.Resource;
import javax.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.List;

/**
 * @Author: Harold澂冰
 * @Date: 2022/7/15 15:27
 */
@RestController
@RequestMapping("/admin")

/*
 * Note: functions in this controller is authenticated before executed, the caller must be an admin
 * to be able to call any of the below functions.
 */
public class AdminController {

    @Resource
    UserIdentity verify;

    @Resource
    IAdminService service;

    /**
     * Function that used to verify if a user is an admin
     * @param request an HttpServletRequest which pass in the user's current access token
     *      example:
     *          (In Header):  (Key: HRD-Token, Value: 41d4d274-286d-4244-8bed-2c9090d62db0)
     * @return a Result object with t=true, msgCode=200 means the user is an admin.
     *      example:
     *          {
     *             "t": true,
     *              "msgCode": 200
     *           }
     */
    @GetMapping("/isAdmin")
    public Result<Boolean> isAdmin(HttpServletRequest request)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(Boolean.FALSE, 403);
        boolean res = service.isAdmin(userId);
        return new Result<>(res, 200);
    }

    /**
     * Function that used to ping MySQL service to see if it works normally
     * @param request an HttpServletRequest which pass in the user's current access token
     * @return a result that contains network code and an integer, if res == 1 and code == 200
     * then MySQL is alive.
     *
     * example return: {
     *     "t": 1,
     *     "msgCode": 200
     * }
     *
     */
    @GetMapping("/pingMySQL")
    public Result<Integer> pingMySQL(HttpServletRequest request)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.pingMySQL(userId);
        return res == 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function that used to ping Redis service to see if it works normally
     * @param request an HttpServletRequest which pass in the user's current access token
     * @return a result that contains network code and an integer, if res == 1 and code == 200
     * then Redis is alive.
     */
    @GetMapping("/pingRedis")
    public Result<Integer> pingRedis(HttpServletRequest request)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.pingRedis(userId);
        return res == 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function used to get MySQL, Redis addresses, upload folder, and config table information
     * @param request an HttpServletRequest which pass in the user's current access token
     * @return example:
     *  {
     *     "t": {
     *         "mysql": "localhost:3306",
     *         "redis": "127.0.0.1:6379",
     *         "upload": "D:/Upload/photoly",
     *         "config": "MySQL: <setting> table"
     *     },
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getAddress")
    public Result<LinkedHashMap<String, String>> getAddress(HttpServletRequest request)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        LinkedHashMap<String, String> map = service.getAddress(userId);
        return map == null ? new Result<>(null, 400) : new Result<>(map, 200);
    }

    /**
     * Function to acquire all settings' entries in the MySQL setting table
     * @param request an HttpServletRequest which pass in the user's current access token
     * @return all entries in the setting table
     * example:
     *  {
     *     "t": {
     *         "ping": "pong",
     *         "SignUp": "1",
     *         "SSafeUUID": "1",
     *         "TokenDuration": "60"
     *     },
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getSettings")
    public Result<LinkedHashMap<String, String>> getSettings(HttpServletRequest request)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        LinkedHashMap<String, String> map = service.getSettings(userId);
        return map == null ? new Result<>(null, 400) : new Result<>(map, 200);
    }

    /**
     * Function to stop the backend server
     * @param request an HttpServletRequest which pass in the user's current access token
     * @return none if successful (since the server will be stopped)
     */
    @PostMapping("/stop")
    public Result<Integer> stopServer(HttpServletRequest request)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        return new Result<>(service.stopServer(userId), 591023);
    }

    /**
     * Function to change signup permission in settings table
     * @param request an HttpServletRequest which pass in the user's current access token
     * @param permission 1 if allowed to sign up, 0 or -1 (basically all integer other than 1) otherwise
     * @return res == 1 and code == 200 if change succeed
     * example:
     *  {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/setSignUpPermission")
    public Result<Integer> setSignUpPermission(HttpServletRequest request, Integer permission)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        if (permission == null)
            return new Result<>(-1, 400);
        int res = service.setSignUpPermission(userId, permission);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function to change SSafeUUIDMode permission in settings table
     * @param request an HttpServletRequest which pass in the user's current access token
     * @param mode 1 if the SSafeUUID Mode is enabled, 0 or -1 (basically all integer other than 1) otherwise
     * @return res == 1 and code == 200 if change succeed
     * example:
     *  {
     *     "t": 1,
     *     "msgCode": 200
     * }
     *
     * Note: SSafeUUID Mode checks for new user's random generated UUID to see if that UUID contradicts with
     * other user's UUID (very very unlikely case) and definitely avoid that circumstance if enabled.
     */
    @PostMapping("/setSSafeUUIDMode")
    public Result<Integer> setSSafeUUIDMode(HttpServletRequest request, Integer mode)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        if (mode == null)
            return new Result<>(-1, 400);
        int res = service.setSSafeUUIDMode(userId, mode);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function to change duration of the user token given after a successful sign in or sign up.
     * @param request an HttpServletRequest which pass in the user's current access token
     * @param days days the token will last in the Redis
     * @return res == 1 and code == 200 if change succeed
     * example: same as above
     */
    @PostMapping("/setTokenDuration")
    public Result<Integer> setTokenDuration(HttpServletRequest request, Integer days)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        if (days == null)
            return new Result<>(-1, 400);
        if (days < 0 || days > 365)
            return new Result<>(-1, 403);
        int res = service.setTokenDuration(userId, days);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function to list users and their information, paged.
     * @param request an HttpServletRequest which pass in the user's current access token
     * @param page page number
     * @param rowsPerPage entry per page
     * @return a result object that contains list of User objects
     * example:
     *  {
     *     "t": [
     *         {
     *             "userId": 4,
     *             "userName": "😀Haha",
     *             "email": "haha@haha.com",
     *             "createDate": "2022-07-02T08:04:50",
     *             "role": "user",
     *             "password": "xxxxxx",
     *             "uuid": "xxxxxx"
     *         }, ...
     *     ],
     *     "pageNum": 1,
     *     "msgCode": 200
     * }
     */
    @GetMapping("/getUserPage")
    public ResultPage<List<User>> getUserPage(HttpServletRequest request, Integer page, Integer rowsPerPage)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new ResultPage<>(null, 0, 403);
        return service.getUserPage(userId, page, rowsPerPage);
    }

    /**
     * Function to list users and their information, paged.
     * @param request an HttpServletRequest which pass in the user's current access token
     * @param page page number
     * @param rowsPerPage entry per page
     * @param search a string that may contain information about an user's username, email, etc.
     * @return a result object that contains list of User objects
     * example: same as above
     */
    @GetMapping("/searchUserPage")
    public ResultPage<List<User>> searchUserPage(HttpServletRequest request, String search, Integer page, Integer rowsPerPage)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new ResultPage<>(null, 0, 403);
        return service.searchUserPage(userId, search, page, rowsPerPage);
    }

    /**
     * Function to add a user manually, role can be specified as "user" (default if not given) or "admin"
     * @param request an HttpServletRequest which pass in the admin's current access token
     * @param user a user object
     *           example input:
     *             {
     *                 "role": "admin",
     *                 "userName": "test2ca",
     *                 "email": "test20240208_2ca@test.com",
     *                 "password": "12345"
     *             }
     * @return res == 1 and code == 200 for success
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/addUser")
    public Result<Integer> addUser(HttpServletRequest request, @RequestBody User user)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.addUser(userId, user);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function to delete a user by its userId
     * @param request an HttpServletRequest which pass in the admin's current access token
     * @param userId the target user's userId
     * @return res >= 1 and code == 200 for success, res == -5 and code == 400 means the user does not exist
     * example: see above
     */
    @PostMapping("/deleteUser")
    public Result<Integer> deleteUser(HttpServletRequest request, Integer userId)
    {
        Integer id = verify.verifyUser(request);
        if (id < 0)
            return new Result<>(-2, 403);
        int res = service.deleteUser(id, userId);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function to reset a user's password
     * @param request an HttpServletRequest which pass in the admin's current access token
     * @param user a user object that must contain userId and password (new).
     * @return res >= 1 and code == 200 for success.
     * example: see above
     */
    @PostMapping("/resetUserPassword")
    public Result<Integer> resetUserPassword(HttpServletRequest request, @RequestBody User user)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.resetUserPassword(userId, user);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function to reset a user's email
     * @param request an HttpServletRequest which pass in the admin's current access token
     * @param user a user object that must contain userId and email (new).
     * @return res >= 1 and code == 200 for success.
     * example: see above
     */
    @PostMapping("/resetUserEmail")
    public Result<Integer> resetUserEmail(HttpServletRequest request, @RequestBody User user)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.resetUserEmail(userId, user);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function to reset a user's role (user or admin)
     * @param request an HttpServletRequest which pass in the admin's current access token
     * @param user a user object that must contain userId and role (new, either "user" or "admin").
     * @return res >= 1 and code == 200 for success.
     * example: see above
     */
    @PostMapping("/resetUserRole")
    public Result<Integer> resetUserRole(HttpServletRequest request, @RequestBody User user)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.resetUserRole(userId, user);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function to reset a user's userName
     * @param request an HttpServletRequest which pass in the admin's current access token
     * @param user a user object that must contain userId and userName (new).
     * @return res >= 1 and code == 200 for success.
     * example: see above
     */
    @PostMapping("/resetUsername")
    public Result<Integer> resetUsername(HttpServletRequest request, @RequestBody User user)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.resetUsername(userId, user);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /**
     * Function to get other setting's value by setting_name in MySQL's setting table
     * @param request an HttpServletRequest which pass in the admin's current access token
     * @param name setting_name
     * @return result of the setting_value of that setting_name
     * example:
     * {
     *     "t": "pong",
     *     "msgCode": 200
     * }
     */
    @PostMapping("/getGeneralSetting")
    public Result<String> getGeneral(HttpServletRequest request, String name)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        String res = service.getGeneralSetting(userId, name);
        return res != null ? new Result<>(res, 200) : new Result<>(null, 400);
    }

    /**
     * Function to set other setting's value by setting_name in MySQL's setting table
     * @param request an HttpServletRequest which pass in the admin's current access token
     * @param name setting_name
     * @param val setting_value
     * @return res == 1 and code == 200 for success
     * example:
     * {
     *     "t": 1,
     *     "msgCode": 200
     * }
     */
    @PostMapping("/setGeneralSetting")
    public Result<Integer> setGeneral(HttpServletRequest request, String name, String val)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(-2, 403);
        int res = service.setGeneralSetting(userId, name, val);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }
}
