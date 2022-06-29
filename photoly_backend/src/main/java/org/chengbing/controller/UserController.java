package org.chengbing.controller;


import org.chengbing.dao.UserMapper;
import org.chengbing.entity.User;
import org.chengbing.service.IGalleryService;
import org.chengbing.service.INamespaceService;
import org.chengbing.service.IUserService;
import org.chengbing.util.AESUtil;
import org.chengbing.util.Result;
import org.chengbing.util.ResultToken;
import org.springframework.data.redis.core.RedisTemplate;
import org.springframework.web.bind.annotation.*;

import javax.annotation.Resource;
import javax.servlet.http.HttpServletRequest;
import java.time.LocalDateTime;
import java.util.Date;
import java.util.UUID;
import java.util.concurrent.TimeUnit;

***REMOVED***
 * <p>
 *  前端控制器
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
@RestController
@RequestMapping("/user")
public class UserController {
    @Resource
    IUserService service;

    @Resource
    INamespaceService namespaceService;

    @Resource
    IGalleryService galleryService;

    @Resource
    RedisTemplate<String, Object> template;

    @PostMapping("/signUp")
    public ResultToken<String> signUp(@RequestBody User user)
***REMOVED***
        user.setUserId(null);
        user.setCreateDate(LocalDateTime.now());
        user.setPassword(AESUtil.aesEncryptStr(user.getPassword(), AESUtil.getPkey()));
        user.setRole("user");
        user.setUuid(UUID.randomUUID().toString());
        if (service.selectUsers(user.getEmail()).size() > 0)
            return new ResultToken<>("Repeated Email Address", null, 401);
        boolean succeed = service.save(user);
        if (succeed) {
            String token = UUID.randomUUID().toString();
            Integer id = service.selectUserByEmail(user.getEmail()).getUserId();
            galleryService.insertGallery("My Favorite", id, "#FFFFFF");
            namespaceService.insertNamespace("/", -1, id);
            template.opsForValue().set(token,id,1, TimeUnit.DAYS);
            return new ResultToken<>("Succeed", token, 200);
***REMOVED***else
            return new ResultToken<>("Error Create User", null, 400);
***REMOVED***

    @PostMapping("/signIn")
    public ResultToken<String> signIn(@RequestBody User user)
***REMOVED***
        User user1 = service.selectUserToLogin(user);
        if (user1.getUserId() != null && user1.getEmail() != null)
    ***REMOVED***
            Integer userId = user1.getUserId();
            String token = UUID.randomUUID().toString();
            template.opsForValue().set(token,userId,1, TimeUnit.DAYS);
            return new ResultToken<>("Succeed", token, 200);
***REMOVED***else{
            return new ResultToken<>("Invalid Username or Email", null, 400);
***REMOVED***
***REMOVED***

    @PostMapping("/signOut")
    public Result<Integer> signOut(HttpServletRequest request)
***REMOVED***
        String key = request.getHeader("HRD-Token");
        template.delete(key);
        return new Result<>(1, 200);
***REMOVED***

    @GetMapping("/getInfo")
    public Result<User> getInfo(HttpServletRequest request)
***REMOVED***
        String key = request.getHeader("HRD-Token");
        if(key == null || key.equals(""))
            return new Result<>(null, 401);
        Integer id = (Integer) template.opsForValue().get(key);
        if (id == null)
            return new Result<>(null, 401);
        User user = service.getById(id);
        if (user == null || user.getEmail() == null || user.getEmail().equals(""))
    ***REMOVED***
            return new Result<>(null, 404);
***REMOVED***else
    ***REMOVED***
            return new Result<>(user, 200);
***REMOVED***
***REMOVED***
***REMOVED***

