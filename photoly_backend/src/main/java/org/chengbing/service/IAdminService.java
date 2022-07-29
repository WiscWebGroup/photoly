package org.chengbing.service;

import org.chengbing.entity.User;
import org.chengbing.util.ResultPage;
import org.springframework.web.bind.annotation.RequestBody;
import java.util.LinkedHashMap;
import java.util.List;

/**
 * @Author: Harold澂冰
 * @Date: 2022/7/15 15:28
 */

public interface IAdminService {
    Boolean isAdmin(Integer userId);
    Integer pingMySQL(Integer adminId);
    Integer pingRedis(Integer adminId);
    LinkedHashMap<String, String> getAddress(Integer adminId);
    LinkedHashMap<String, String> getSettings(Integer adminId);
    Integer stopServer(Integer adminId);
    Integer setSignUpPermission(Integer adminId, Integer permission);
    Integer setSSafeUUIDMode(Integer adminId, Integer mode);
    Integer setTokenDuration(Integer adminId, Integer days);

    ResultPage<List<User>> getUserPage(Integer adminId, Integer page, Integer rowsPerPage);

    ResultPage<List<User>> searchUserPage(Integer adminId, String search, Integer page, Integer rowsPerPage);

    Integer addUser(Integer adminId, @RequestBody User user);

    Integer deleteUser(Integer adminId, Integer userId);


    Integer resetUserPassword(Integer adminId, @RequestBody User user);

    Integer resetUserEmail(Integer adminId, @RequestBody User user);
    Integer resetUserRole(Integer adminId, @RequestBody User user);
    Integer resetUsername(Integer adminId, @RequestBody User user);

    String getGeneralSetting(Integer adminId, String name);

    Integer setGeneralSetting(Integer adminId, String name, String val);
}
