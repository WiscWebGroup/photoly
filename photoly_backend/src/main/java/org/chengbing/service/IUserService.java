package org.chengbing.service;

import org.chengbing.entity.User;
import com.baomidou.mybatisplus.extension.service.IService;

import java.util.List;

***REMOVED***
 * <p>
 *  服务类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
public interface IUserService extends IService<User> {
    List<User> selectUsers(String email);
    User selectUserByEmail(String email);
    User selectUserToLogin(User user);
***REMOVED***
