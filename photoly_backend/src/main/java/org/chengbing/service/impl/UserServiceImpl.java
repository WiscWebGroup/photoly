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
import org.springframework.web.multipart.MultipartFile;

import javax.annotation.Resource;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileNotFoundException;
import java.io.IOException;
import java.util.List;

***REMOVED***
 * <p>
 *  服务实现类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
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

    @Value("${file.uploadFolder***REMOVED***")
    String uploadFolder;

    @Override
    public List<User> selectUsers(String email) {
        QueryWrapper<User> wrapper = new QueryWrapper<>();
        wrapper.eq("email", email);
        return mapper.selectList(wrapper);
***REMOVED***

    @Override
    public User selectUserByEmail(String email) {
        QueryWrapper<User> wrapper = new QueryWrapper<>();
        wrapper.eq("email", email);
        return mapper.selectOne(wrapper);
***REMOVED***

    @Override
    public User selectUserToLogin(User user) {
        String email = user.getEmail();
        String password = AESUtil.aesEncryptStr(user.getPassword(), AESUtil.getPkey());
        QueryWrapper<User> wrapper = new QueryWrapper<>();
        wrapper.eq("password", password);
        wrapper.eq("email", email);
        return mapper.selectOne(wrapper);
***REMOVED***

    @Override
    public Integer updateUsername(Integer userId, String username) {
        UpdateWrapper<User> updateWrapper = new UpdateWrapper<>();
        updateWrapper.eq("user_id", userId);
        updateWrapper.set("user_name", username);
        return mapper.update(null, updateWrapper);
***REMOVED***

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
***REMOVED***

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
***REMOVED***

    @Override
    public Integer updateAvatar(Integer userId, MultipartFile img) {
        String uuid = mapper.selectById(userId).getUuid();
        String folder = uploadFolder + System.getProperty("file.separator") + uuid;
        File folderDir = new File(folder);
        if (!folderDir.exists())
            folderDir.mkdir();
        File file = new File(uploadFolder + System.getProperty("file.separator") + uuid + System.getProperty("file.separator") + uuid + ".jpg");
***REMOVED***
            img.transferTo(file);
            return 1;
***REMOVED*** catch (IOException e) {
            throw new RuntimeException(e);
***REMOVED***
***REMOVED***

    @Override
    public byte[] getAvatar(Integer userId) {
        String uuid = mapper.selectById(userId).getUuid();
        File file = new File(uploadFolder + System.getProperty("file.separator") + uuid + System.getProperty("file.separator") + uuid + ".jpg");
        FileInputStream inputStream = null;
***REMOVED***
            inputStream = new FileInputStream(file);
            byte[] bytes = new byte[inputStream.available()];
            inputStream.read(bytes, 0, inputStream.available());
            return bytes;
***REMOVED*** catch (IOException e) {
            throw new RuntimeException(e);
***REMOVED***
***REMOVED***

    @Override
    public Integer deleteAccount(Integer operatorId, Integer deleteUserId) {
        QueryWrapper<User> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("user_id", operatorId);
        User operator = mapper.selectOne(queryWrapper);
        if (operator!= null && (operator.getUserId().equals(deleteUserId) || operator.getRole().equals("admin")))
    ***REMOVED***
            // Delete Namespace
            Namespace root = namespaceService.queryRootNamespace(deleteUserId);
            int deleteNamespace = namespaceService.deleteNamespace(deleteUserId, root.getNsId());
            // Delete Tag
            QueryWrapper<Tag> tagQueryWrapper = new QueryWrapper<>();
            tagQueryWrapper.eq("user_id", deleteUserId);
            int deleteTag = tagMapper.delete(tagQueryWrapper);
            // Delete Gallery
            QueryWrapper<Gallery> galleryQueryWrapper = new QueryWrapper<>();
            tagQueryWrapper.eq("user_id", deleteUserId);
            int deleteGallery = galleryMapper.delete(galleryQueryWrapper);
            return deleteNamespace + deleteTag + deleteGallery;
***REMOVED***else{
            return -2;
***REMOVED***
***REMOVED***
***REMOVED***
