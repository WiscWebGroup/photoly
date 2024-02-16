package org.chengbing.controller;


import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import org.chengbing.dao.SettingMapper;
import org.chengbing.dao.UserMapper;
import org.chengbing.entity.Photo;
import org.chengbing.entity.Setting;
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
    SettingMapper settingMapper;

    @Resource
    RedisTemplate<String, Object> template;

    @Resource
    UserIdentity verify;

    /***
     * Function used to sign up users.
     * @param user: an JSON serialized User object with userName, email and password
     *            example:
     *                  {
     *                     "userName": "test20240202_1",
     *                     "email": "test20240202_1@test.com",
     *                     "password": "hello123"
     *                   }
     * @return an ResultToken object, 200 means succeed, other means failed. Message is included.
     */
    @PostMapping("/signUp")
    public ResultToken<String> signUp(@RequestBody User user)
    {
        QueryWrapper<Setting> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("setting_name", "SignUp");
        Setting setting = settingMapper.selectOne(queryWrapper);

        // if the "SignUp" setting value in the database is 1, that means that the admin has enabled the sign up
        // function, so that the program goes to proceed to sign up for the user.
        if (setting.getSettingValue().equals("1"))
        {
            String email = user.getEmail();

            // if the email address is null, then return an 400 error.
            if (email == null)
                return new ResultToken<>("Email is null", null, 400);

            // check if the email format is correct. If not, return an 400 error.
            Pattern pattern = Pattern.compile(".+@.+\\..+");
            Matcher matcher = pattern.matcher(email);
            if (!matcher.find())
                return new ResultToken<>("Email is incorrect", null, 400);

            // force to clear the userId (avoid injection), and encrypt the user's password.
            user.setUserId(null);
            user.setCreateDate(LocalDateTime.now());
            user.setPassword(AESUtil.aesEncryptStr(user.getPassword(), AESUtil.getPkey()));
            user.setRole("user");

            String userUUID = UUID.randomUUID().toString();

            QueryWrapper<Setting> queryWrapper2 = new QueryWrapper<>();
            queryWrapper2.eq("setting_name", "SSafeUUID");
            Setting setting2 = settingMapper.selectOne(queryWrapper2);

            // If the SSafe UUID mode is enabled, make sure that the new UUID for the user will
            // not be the UUID of any other user (although very unlikely).
            if (setting2.getSettingValue().equals("1"))
            {
                boolean verified = false;
                while (!verified)
                {
                    QueryWrapper<User> userQueryWrapper = new QueryWrapper<>();
                    userQueryWrapper.eq("uuid", userUUID);
                    if (service.list(userQueryWrapper).size() >= 1)
                        userUUID = UUID.randomUUID().toString();
                    else
                        verified = true;
                }
            }

            user.setUuid(userUUID);

            // Check if the user's email is already registered by other.
            if (service.selectUsers(user.getEmail()).size() > 0)
                return new ResultToken<>("Repeated Email Address", null, 401);
            boolean succeed = service.save(user);

            // If successful, do some default settings and use the Redis Template to insert the login token
            // into the designated Redis database.
            if (succeed) {
                String token = UUID.randomUUID().toString();
                Integer id = service.selectUserByEmail(user.getEmail()).getUserId();
                galleryService.insertGallery("My Favorite", id, "#FFFFFF");
                namespaceService.insertNamespace("/", -1, id);

                QueryWrapper<Setting> queryWrapper3 = new QueryWrapper<>();
                queryWrapper3.eq("setting_name", "TokenDuration");
                Setting setting3 = settingMapper.selectOne(queryWrapper3);

                template.opsForValue().set(token,id,Integer.parseInt(setting3.getSettingValue()), TimeUnit.DAYS);
                return new ResultToken<>("Succeed", token, 200);
            }else
                return new ResultToken<>("Error Create User", null, 400);
        }else {
            return new ResultToken<>("Register is closed by Admin", null, 400);
        }
    }

    /***
     * Function used to sign in users.
     * @param user: an JSON serialized User object with email and password
     *            example:
     *                  {
     *                     "email": "test20240202_1@test.com",
     *                     "password": "hello123"
     *                   }
     * @return an ResultToken object, 200 means succeed, other means failed. Message is included.
     */
    @PostMapping("/signIn")
    public ResultToken<String> signIn(@RequestBody User user)
    {
        User user1 = service.selectUserToLogin(user);

        // insert into the Redis database with the login token for this user
        if (user1!=null && user1.getUserId() != null && user1.getEmail() != null)
        {
            Integer userId = user1.getUserId();
            String token = UUID.randomUUID().toString();

            QueryWrapper<Setting> queryWrapper3 = new QueryWrapper<>();
            queryWrapper3.eq("setting_name", "TokenDuration");
            Setting setting3 = settingMapper.selectOne(queryWrapper3);

            template.opsForValue().set(token,userId,Integer.parseInt(setting3.getSettingValue()), TimeUnit.DAYS);
            return new ResultToken<>("Succeed", token, 200);
        }else{
            return new ResultToken<>("Invalid Email or Password", null, 400);
        }
    }

    /***
     * Function used to sign out users.
     * @param request an HttpServletRequest which pass in the user's current access token.
     *            example:
     *               (In Header):  (Key: HRD-Token, Value: 41d4d274-286d-4244-8bed-2c9090d62db0)
     * @return an ResultToken object, 200 means succeed, other means failed. Message is included.
     */
    @PostMapping("/signOut")
    public Result<Integer> signOut(HttpServletRequest request)
    {
        String key = request.getHeader("HRD-Token");
        template.delete(key);
        return new Result<>(1, 200);
    }

    /***
     * Function used to get user's information.
     * @param request an HttpServletRequest which pass in the user's current access token.
     *            example:
     *               (In Header):  (Key: HRD-Token, Value: 41d4d274-286d-4244-8bed-2c9090d62db0)
     * @return an Result object, 200 means succeed, other means failed. Message is included. User object is included.
     * please note that password will not be included into this query.
     *      example:
     *          {
     *             "t": {
     *             "userId": 39,
     *             "userName": "test20240202_1",
     *             "email": "test20240202_1@test.com",
     *             "createDate": "2024-02-02T16:34:27",
     *             "role": "user",
     *             "password": null,
     *             "uuid": "188f9163-493b-47a5-bca5-4fab21ffaa46"
     *         },
     *     "msgCode": 200
     *           }
     */
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

    /***
     * Function used to update user's username.
     * @param request an HttpServletRequest which pass in the user's current access token.
     *            example:
     *               (In Header):  (Key: HRD-Token, Value: 41d4d274-286d-4244-8bed-2c9090d62db0)
     *        username a string of new username
     *                example:
     *                  "ad977暴暴龙"
     * @return a Result object, 200 means succeed, other means failed. Message is included.
     */
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

    /***
     * Function used to update user's email.
     * @param request an HttpServletRequest which pass in the user's current access token.
     *            example:
     *               (In Header):  (Key: HRD-Token, Value: 41d4d274-286d-4244-8bed-2c9090d62db0)
     *        email a string of new email
     *                example:
     *                  "ad977@baobaolong.com"
     * @return a Result object, 200 means succeed, other means failed. Message is included.
     */
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

    /***
     * Function used to update user's password.
     * @param request an HttpServletRequest which pass in the user's current access token.
     *            example:
     *               (In Header):  (Key: HRD-Token, Value: 41d4d274-286d-4244-8bed-2c9090d62db0)
     *        oldPass a string of the old password
     *                example:
     *                  "hello123"
     *        newPass a string of new password
     *                example:
     *                  "hello1234"
     * @return a Result object, 200 means succeed, other means failed. Message is included.
     */
    @PostMapping("/updatePassword")
    public Result<Integer> updatePassword(HttpServletRequest request, String oldPass, String newPass)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updatePassword(userId, oldPass, newPass);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /***
     * Function used to update user's avatar.
     * @param request an HttpServletRequest which pass in the user's current access token.
     *            example:
     *               (In Header):  (Key: HRD-Token, Value: 41d4d274-286d-4244-8bed-2c9090d62db0)
     *        file a MultipartFile object
     *                example:
     *                  (form-data): (Key: file, Value: [a file selected])
     * @return a Result object, 200 means succeed, other means failed. Message is included.
     */
    @PostMapping("/updateAvatar")
    public Result<Integer> updateAvatar(HttpServletRequest request, MultipartFile file)
    {
        Integer userId = verify.verifyUser(request);
        if (userId < 0)
            return new Result<>(null, 403);
        int change = service.updateAvatar(userId, file);
        return change == 1 ? new Result<>(change, 200) : new Result<>(change, 400);
    }

    /***
     * Function used to get user's avatar via authenticate token that gave after the login.
     * @param token a authenticate token that gave after the login.
     *            example:
     *               {url}/user/getAvatar/41d4d274-286d-4244-8bed-2c9090d62db0
     * @return an array of byte that encode the user's avatar.
     */
    @GetMapping(value="/getAvatar/{token}", produces = MediaType.IMAGE_JPEG_VALUE)
    public byte[] getAvatar(@PathVariable String token)
    {
        Integer userId = verify.verifyUserByToken(token);
        if (userId < 0)
            return null;
        return service.getAvatar(userId);
    }

    /***
     * Function used to remove the user completely. Use this function with extreme caution.
     * @param request an HttpServletRequest which pass in the user's current access token.
     *            example:
     *               (In Header):  (Key: HRD-Token, Value: 41d4d274-286d-4244-8bed-2c9090d62db0)
     * @return an Result<Integer>, res=1 and code=200 means successful.
     */
    @GetMapping("/deleteUser")
    public Result<Integer> deleteUser(HttpServletRequest request)
    {
        Integer id = verify.verifyUser(request);
        if (id < 0)
            return new Result<>(null, 403);
        int res = service.deleteAccount(id, id);
        return res >= 1 ? new Result<>(res, 200) : new Result<>(res, 400);
    }

    /***
     * Function used to see if a token is still alive.
     * @param request an HttpServletRequest which pass in the user's current access token.
     *            example:
     *               (In Header):  (Key: HRD-Token, Value: 41d4d274-286d-4244-8bed-2c9090d62db0)
     * @return an Result<Integer>, res=1 and code=200 means successful.
     */
    @PostMapping("/pingToken")
    public Result<Integer> pingToken(HttpServletRequest request)
    {
        Integer id = verify.verifyUser(request);
        if (id < 0)
            return new Result<>(null, 403);

        return new Result<>(1, 200);
    }
}

