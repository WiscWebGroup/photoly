package org.chengbing.service.impl;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import org.chengbing.dao.GalleryMapper;
import org.chengbing.dao.TagMapper;
import org.chengbing.entity.Gallery;
import org.chengbing.entity.Namespace;
import org.chengbing.entity.Tag;
import org.chengbing.entity.User;
import org.chengbing.dao.UserMapper;
import org.chengbing.service.INamespaceService;
import org.chengbing.service.IUserService;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import org.chengbing.util.AESUtil;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.util.ResourceUtils;
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

    @Resource
    INamespaceService namespaceService;

    @Resource
    GalleryMapper galleryMapper;

    @Resource
    TagMapper tagMapper;

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
            inputStream.close();
            return bytes;
        } catch (IOException e) {
            // throw new RuntimeException(e);
            try {
                inputStream = new FileInputStream(ResourceUtils.getFile("classpath:defaultAvatar.jpg"));
                byte[] bytes = new byte[inputStream.available()];
                inputStream.read(bytes, 0, inputStream.available());
                inputStream.close();
                return bytes;
            }catch (IOException ex) {
                throw new RuntimeException(ex);
            }
        }
    }

    @Override
    public Integer deleteAccount(Integer operatorId, Integer deleteUserId) {
        QueryWrapper<User> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("user_id", operatorId);
        User operator = mapper.selectOne(queryWrapper);

        User goodbye_and_good_luck = mapper.selectById(deleteUserId);

        // if the user does not exist
        if (goodbye_and_good_luck == null)
        {
            return -5;
        }

        // if exist then check
        if (operator!= null && (operator.getUserId().equals(deleteUserId) || operator.getRole().equals("admin")))
        {
            // Delete Namespace
            String UUID = goodbye_and_good_luck.getUuid();
            Namespace root = namespaceService.queryRootNamespace(deleteUserId);
            int deleteNamespace = namespaceService.deletePhotoHelper(deleteUserId, root.getNsId(), UUID);
            // Delete Tag
            QueryWrapper<Tag> tagQueryWrapper = new QueryWrapper<>();
            tagQueryWrapper.eq("user_id", deleteUserId);
            int deleteTag = tagMapper.delete(tagQueryWrapper);
            // Delete Gallery
            QueryWrapper<Gallery> galleryQueryWrapper = new QueryWrapper<>();
            galleryQueryWrapper.eq("user_id", deleteUserId);
            int deleteGallery = galleryMapper.delete(galleryQueryWrapper);
            // Delete Account
            int deleteUser = mapper.deleteById(deleteUserId);
            return deleteNamespace + deleteTag + deleteGallery + deleteUser;
        }else{
            return -2;
        }
    }
}
