package org.chengbing.service.impl;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import org.chengbing.entity.User;
import org.chengbing.dao.UserMapper;
import org.chengbing.service.IUserService;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import org.chengbing.util.AESUtil;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.web.multipart.MultipartFile;

import javax.annotation.Resource;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileNotFoundException;
import java.io.IOException;
import java.util.List;

/**
 * <p>
 *  服务实现类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@Service
public class UserServiceImpl extends ServiceImpl<UserMapper, User> implements IUserService {

    @Resource
    UserMapper mapper;

    @Value("${file.uploadFolder}")
    String uploadFolder;

    @Override
    public List<User> selectUsers(String email) {
        QueryWrapper<User> wrapper = new QueryWrapper<>();
        wrapper.eq("email", email);
        return mapper.selectList(wrapper);
    }

    @Override
    public User selectUserByEmail(String email) {
        QueryWrapper<User> wrapper = new QueryWrapper<>();
        wrapper.eq("email", email);
        return mapper.selectOne(wrapper);
    }

    @Override
    public User selectUserToLogin(User user) {
        String email = user.getEmail();
        String password = AESUtil.aesEncryptStr(user.getPassword(), AESUtil.getPkey());
        QueryWrapper<User> wrapper = new QueryWrapper<>();
        wrapper.eq("password", password);
        wrapper.eq("email", email);
        return mapper.selectOne(wrapper);
    }

    @Override
    public Integer updateUsername(Integer userId, String username) {
        UpdateWrapper<User> updateWrapper = new UpdateWrapper<>();
        updateWrapper.eq("user_id", userId);
        updateWrapper.set("user_name", username);
        return mapper.update(null, updateWrapper);
    }

    @Override
    public Integer updateEmail(Integer userId, String email) {
        QueryWrapper<User> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("email", email);
        List<User> data = mapper.selectList(queryWrapper);
        if (data.size() > 0)
            return -2;

        UpdateWrapper<User> updateWrapper = new UpdateWrapper<>();
        updateWrapper.eq("user_id", userId);
        updateWrapper.set("email", email);
        return mapper.update(null, updateWrapper);
    }

    @Override
    public Integer updatePassword(Integer userId, String oldPass, String newPass) {
        QueryWrapper<User> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("user_id", userId);
        queryWrapper.eq("password", AESUtil.aesEncryptStr(oldPass, AESUtil.getPkey()));
        User data = mapper.selectOne(queryWrapper);
        if (data == null)
            return -2;

        UpdateWrapper<User> updateWrapper = new UpdateWrapper<>();
        updateWrapper.eq("user_id", userId);
        updateWrapper.set("password", AESUtil.aesEncryptStr(newPass, AESUtil.getPkey()));
        return mapper.update(null, updateWrapper);
    }

    @Override
    public Integer updateAvatar(Integer userId, MultipartFile img) {
        String uuid = mapper.selectById(userId).getUuid();
        String folder = uploadFolder + System.getProperty("file.separator") + uuid;
        File folderDir = new File(folder);
        if (!folderDir.exists())
            folderDir.mkdir();
        File file = new File(uploadFolder + System.getProperty("file.separator") + uuid + System.getProperty("file.separator") + uuid + ".jpg");
        try {
            img.transferTo(file);
            return 1;
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    @Override
    public byte[] getAvatar(Integer userId) {
        String uuid = mapper.selectById(userId).getUuid();
        File file = new File(uploadFolder + System.getProperty("file.separator") + uuid + System.getProperty("file.separator") + uuid + ".jpg");
        FileInputStream inputStream = null;
        try {
            inputStream = new FileInputStream(file);
            byte[] bytes = new byte[inputStream.available()];
            inputStream.read(bytes, 0, inputStream.available());
            return bytes;
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
    }
}
