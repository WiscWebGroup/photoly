package org.chengbing.controller;


import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import org.chengbing.dao.UserMapper;
import org.chengbing.entity.Photo;
import org.chengbing.entity.User;
import org.chengbing.service.IGalleryService;
import org.chengbing.service.INamespaceService;
import org.chengbing.service.IUserService;
import org.chengbing.util.AESUtil;
import org.chengbing.util.Result;
import org.chengbing.util.ResultToken;
import org.chengbing.util.UserIdentity;
import org.springframework.data.redis.core.RedisTemplate;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.multipart.MultipartFile;

import javax.annotation.Resource;
import javax.servlet.http.HttpServletRequest;
import java.time.LocalDateTime;
import java.util.Date;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * <p>
 *  前端控制器
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
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

    @Resource
    UserIdentity verify;

    @PostMapping("/signUp")
    public ResultToken<String> signUp(@RequestBody User user)
    {
        String email = user.getEmail();
        if (email == null)
            return new ResultToken<>("Email is null", null, 400);
        Pattern pattern = Pattern.compile(".+@.+\\..+");
        Matcher matcher = pattern.matcher(email);
        if (!matcher.find())
            return new ResultToken<>("Email is incorrect", null, 400);
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
        }else
            return new ResultToken<>("Error Create User", null, 400);
    }

    @PostMapping("/signIn")
    public ResultToken<String> signIn(@RequestBody User user)
    {
        User user1 = service.selectUserToLogin(user);
        if (user1.getUserId() != null && user1.getEmail() != null)
        {
            Integer userId = user1.getUserId();
            String token = UUID.randomUUID().toString();
            template.opsForValue().set(token,userId,1, TimeUnit.DAYS);
            return new ResultToken<>("Succeed", token, 200);
        }else{
            return new ResultToken<>("Invalid Email or Password", null, 400);
        }
    }

    @PostMapping("/signOut")
    public Result<Integer> signOut(HttpServletRequest request)
    {
        String key = request.getHeader("HRD-Token");
        template.delete(key);
        return new Result<>(1, 200);
    }

    @GetMapping("/getInfo")
    public Result<User> getInfo(HttpServletRequest request)
    {
        String key = request.getHeader("HRD-Token");
        if(key == null || key.equals(""))
            return new Result<>(null, 403);
        Integer id = (Integer) template.opsForValue().get(key);
        if (id == null)
            return new Result<>(null, 403);
        User user = service.getById(id);
        if (user == null || user.getEmail() == null || user.getEmail().equals(""))
        {
            return new Result<>(null, 404);
        }else
        {
            user.setPassword(null);
            return new Result<>(user, 200);
        }
    }

    @PostMapping("/updateUsername")
    public Result<Integer> updateUsername(HttpServletRequest request, String username)
    {
        if (username == null || username.equals(""))
            return  new Result<>(-1, 400);
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updateUsername(userId, username);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @PostMapping("/updateEmail")
    public Result<Integer> updateEmail(HttpServletRequest request, String email)
    {
        if (email == null)
            return new Result<>(-1,  400);
        Pattern pattern = Pattern.compile(".+@.+\\..+");
        Matcher matcher = pattern.matcher(email);
        if (!matcher.find())
            return new Result<>(-2,  400);
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updateEmail(userId, email);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @PostMapping("/updatePassword")
    public Result<Integer> updatePassword(HttpServletRequest request, String oldPass, String newPass)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updatePassword(userId, oldPass, newPass);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @PostMapping("/updateAvatar")
    public Result<Integer> updateAvatar(HttpServletRequest request, MultipartFile file)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updateAvatar(userId, file);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    @GetMapping(value="/getAvatar/{token}", produces = MediaType.IMAGE_JPEG_VALUE)
    public byte[] getAvatar(@PathVariable String token)
    {
        Integer userId = verify.verifyUserByToken(token);
        if (userId < 0)
            return null;
        return service.getAvatar(userId);
    }

    @GetMapping("/deleteUser")
    public Result<List<User>> deleteUser(HttpServletRequest request, @RequestBody Integer userId)
    {
        return null;
    }
}

