package org.chengbing.service.impl;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import com.baomidou.mybatisplus.core.metadata.IPage;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.swagger.v3.oas.annotations.links.Link;
import org.chengbing.dao.SettingMapper;
import org.chengbing.dao.UserMapper;
import org.chengbing.entity.*;
import org.chengbing.service.IAdminService;
import org.chengbing.service.IGalleryService;
import org.chengbing.service.INamespaceService;
import org.chengbing.service.IUserService;
import org.chengbing.util.AESUtil;
import org.chengbing.util.ResultPage;
import org.chengbing.util.ResultToken;
import org.springframework.beans.BeansException;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.SpringApplication;
import org.springframework.context.ApplicationContext;
import org.springframework.context.ApplicationContextAware;
import org.springframework.context.ConfigurableApplicationContext;
import org.springframework.stereotype.Service;
import redis.clients.jedis.Jedis;

import javax.annotation.Resource;
import java.time.LocalDateTime;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Objects;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

***REMOVED***
 * @Author: Harold澂冰
 * @Date: 2022/7/15 15:28
***REMOVED***
@Service
public class AdminServiceImpl implements IAdminService, ApplicationContextAware {

    @Resource
    UserMapper userMapper;

    @Resource
    IUserService userService;

    @Resource
    IGalleryService galleryService;

    @Resource
    INamespaceService namespaceService;

    @Resource
    SettingMapper settingMapper;

    @Value("${spring.redis.host***REMOVED***")
    String redisHost;

    @Value("${spring.redis.port***REMOVED***")
    String redisPort;

    @Value("${spring.redis.password***REMOVED***")
    String redisPass;

    @Value("${spring.datasource.url***REMOVED***")
    String mysqlUrl;

    @Value("${file.uploadFolder***REMOVED***")
    String uploadFolder;

    private ApplicationContext context;

    @Override
    public void setApplicationContext(ApplicationContext applicationContext) throws BeansException {
        this.context = applicationContext;
***REMOVED***


    @Override
    public Boolean isAdmin(Integer userId) {
        QueryWrapper<User> wrapper = new QueryWrapper<>();
        wrapper.eq("user_id", userId);
        User user = userService.getOne(wrapper);
        if (user == null || user.getRole() == null)
            return Boolean.FALSE;
        if (user.getRole().equals("admin"))
            return Boolean.TRUE;
        return Boolean.FALSE;
***REMOVED***

    @Override
    public Integer pingMySQL(Integer adminId) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
            return -2;
        QueryWrapper<Setting> wrapper = new QueryWrapper<>();
        wrapper.eq("setting_name", "ping");
        Setting setting = settingMapper.selectOne(wrapper);
        if (setting == null || setting.getSettingValue() == null || !setting.getSettingValue().equals("pong"))
            return -1;
        return 1;
***REMOVED***

    @Override
    public Integer pingRedis(Integer adminId) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
            return -2;
        Jedis jedis = new Jedis(redisHost, Integer.parseInt(redisPort));
        jedis.auth(redisPass);
        String ping = jedis.ping();
        if (ping.equalsIgnoreCase("PONG")) {
            return 1;
***REMOVED***
        return -1;
***REMOVED***

    @Override
    public LinkedHashMap<String, String> getAddress(Integer adminId) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
    ***REMOVED***
        Pattern pattern = Pattern.compile("//(.*?)/");
        Matcher matcher = pattern.matcher(mysqlUrl);
        String mysqlAddress = "";
        while (matcher.find()) {
            mysqlAddress = matcher.group(1);
***REMOVED***
        String redisAddress = redisHost + ":" + redisPort;
        String configAddress = "MySQL: <setting> table";
        String upload = uploadFolder;
        LinkedHashMap<String, String> map = new LinkedHashMap<>();
        map.put("mysql", mysqlAddress);
        map.put("redis", redisAddress);
        map.put("upload", upload);
        map.put("config", configAddress);
        return map;
***REMOVED***

    @Override
    public LinkedHashMap<String, String> getSettings(Integer adminId) {
        QueryWrapper<Setting> wrapper = new QueryWrapper<>();
        List<Setting> settings = settingMapper.selectList(wrapper);
        LinkedHashMap<String, String> map = new LinkedHashMap<>();
        for (Setting setting : settings)
            map.put(setting.getSettingName(), setting.getSettingValue());
        return map;
***REMOVED***

    @Override
    public Integer stopServer(Integer adminId) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
            return -2;
        ((ConfigurableApplicationContext) context).close();
        return 1;
***REMOVED***

    @Override
    public Integer setSignUpPermission(Integer adminId, Integer permission) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
            return -2;
        UpdateWrapper<Setting> updateWrapper = new UpdateWrapper<>();
        updateWrapper.eq("setting_name", "SignUp");
        updateWrapper.set("setting_value", String.valueOf(permission));
        return settingMapper.update(null, updateWrapper);
***REMOVED***

    @Override
    public Integer setSSafeUUIDMode(Integer adminId, Integer mode) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
            return -2;
        UpdateWrapper<Setting> updateWrapper = new UpdateWrapper<>();
        updateWrapper.eq("setting_name", "SSafeUUID");
        updateWrapper.set("setting_value", String.valueOf(mode));
        return settingMapper.update(null, updateWrapper);
***REMOVED***

    @Override
    public Integer setTokenDuration(Integer adminId, Integer days) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
            return -2;
        UpdateWrapper<Setting> updateWrapper = new UpdateWrapper<>();
        updateWrapper.eq("setting_name", "TokenDuration");
        updateWrapper.set("setting_value", String.valueOf(days));
        return settingMapper.update(null, updateWrapper);
***REMOVED***

    @Override
    public ResultPage<List<User>> getUserPage(Integer adminId, Integer page, Integer rowsPerPage) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
            return new ResultPage<>(null, 0, 400);
        QueryWrapper<User> wrapper = new QueryWrapper<>();
        Page<User> userPage = new Page<>(page, rowsPerPage);
        IPage<User> users = userMapper.selectPage(userPage, wrapper);
        return new ResultPage<>(users.getRecords(), Math.toIntExact(users.getPages()), 200);
***REMOVED***

    @Override
    public ResultPage<List<User>> searchUserPage(Integer adminId, String search, Integer page, Integer rowsPerPage) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
            return new ResultPage<>(null, 0, 400);
        QueryWrapper<User> wrapper = new QueryWrapper<>();
        wrapper.like("user_name", search).or().like("email", search);
        Page<User> userPage = new Page<>(page, rowsPerPage);
        IPage<User> users = userMapper.selectPage(userPage, wrapper);
        return new ResultPage<>(users.getRecords(), Math.toIntExact(users.getPages()), 200);
***REMOVED***

    @Override
    public Integer addUser(Integer adminId, User user) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
            return -2;
        if (user == null || user.getUserName() == null || user.getEmail() == null || user.getPassword() == null)
            return -1;
        if (userService.selectUsers(user.getEmail()).size() > 0)
            return -1;
        if (user.getRole() == null || (!user.getRole().equals("user") && !user.getRole().equals("admin")))
            user.setRole("user");
        user.setUserId(null);
        user.setCreateDate(LocalDateTime.now());
        user.setPassword(AESUtil.aesEncryptStr(user.getPassword(), AESUtil.getPkey()));
        user.setUuid(UUID.randomUUID().toString());
        boolean succeed = userService.save(user);
        if (succeed) {
            Integer id = userService.selectUserByEmail(user.getEmail()).getUserId();
            galleryService.insertGallery("My Favorite", id, "#FFFFFF");
            namespaceService.insertNamespace("/", -1, id);
            return 1;
***REMOVED***
        return -1;
***REMOVED***

    @Override
    public Integer deleteUser(Integer adminId, Integer userId) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
            return -2;
        if (userId == null)
            return -2;
        return userService.removeById(userId) ? 1 : 0;
***REMOVED***

    @Override
    public Integer resetUserPassword(Integer adminId, User user) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
            return -2;
        UpdateWrapper<User> updateWrapper = new UpdateWrapper<>();
        updateWrapper.eq("user_id", user.getUserId());
        updateWrapper.set("password",AESUtil.aesEncryptStr(user.getPassword(), AESUtil.getPkey()));
        return userMapper.update(null, updateWrapper);
***REMOVED***

    @Override
    public Integer resetUserEmail(Integer adminId, User user) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
            return -2;
        List<User> users = userService.selectUsers(user.getEmail());
        if (users.size() == 1 && Objects.equals(users.get(0).getUserId(), user.getUserId()))
            return 1;
        if (userService.selectUsers(user.getEmail()).size() > 0)
            return -1;
        UpdateWrapper<User> updateWrapper = new UpdateWrapper<>();
        updateWrapper.eq("user_id", user.getUserId());
        updateWrapper.set("email", user.getEmail());
        return userMapper.update(null, updateWrapper);
***REMOVED***

    @Override
    public Integer resetUserRole(Integer adminId, User user) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
            return -2;
        if (userService.selectUsers(user.getEmail()).size() > 0)
            return -1;
        UpdateWrapper<User> updateWrapper = new UpdateWrapper<>();
        updateWrapper.eq("user_id", user.getUserId());
        updateWrapper.set("role", user.getRole());
        return userMapper.update(null, updateWrapper);
***REMOVED***

    @Override
    public Integer resetUsername(Integer adminId, User user) {
        boolean isAdmin = isAdmin(adminId);
        if (!isAdmin)
            return -2;
        if (userService.selectUsers(user.getEmail()).size() > 0)
            return -1;
        UpdateWrapper<User> updateWrapper = new UpdateWrapper<>();
        updateWrapper.eq("user_id", user.getUserId());
        updateWrapper.set("user_name", user.getUserName());
        return userMapper.update(null, updateWrapper);
***REMOVED***

***REMOVED***
