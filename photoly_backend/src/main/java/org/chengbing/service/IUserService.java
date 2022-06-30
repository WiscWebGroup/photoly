package org.chengbing.service;

import org.chengbing.entity.User;
import com.baomidou.mybatisplus.extension.service.IService;
import org.springframework.web.multipart.MultipartFile;

import java.util.List;

/**
 * <p>
 *  服务类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
public interface IUserService extends IService<User> {
    List<User> selectUsers(String email);
    User selectUserByEmail(String email);
    User selectUserToLogin(User user);

    Integer updateUsername(Integer userId, String username);
    Integer updateEmail(Integer userId, String email);
    Integer updatePassword(Integer userId, String oldPass, String newPass);
    Integer updateAvatar(Integer userId, MultipartFile img);
    byte[] getAvatar(Integer userId);
}
