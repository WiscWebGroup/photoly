package org.chengbing.service.impl;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import org.chengbing.dao.PhotoMapper;
import org.chengbing.entity.Cred;
import org.chengbing.dao.CredMapper;
import org.chengbing.entity.Namespace;
import org.chengbing.entity.Photo;
import org.chengbing.service.ICredService;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import org.chengbing.service.INamespaceService;
import org.chengbing.service.IPhotoService;
import org.chengbing.service.IUserService;
import org.chengbing.util.Result;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileNotFoundException;
import java.io.IOException;
import java.util.List;
import java.util.UUID;

***REMOVED***
 * <p>
 *  服务实现类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
@Service
public class CredServiceImpl extends ServiceImpl<CredMapper, Cred> implements ICredService {

    @Resource
    CredMapper mapper;

    @Resource
    PhotoMapper photoMapper;

    @Resource
    IPhotoService photoService;

    @Value("${file.uploadFolder***REMOVED***")
    String uploadFolder;

    @Resource
    IUserService userService;

    @Resource
    INamespaceService namespaceService;

    @Override
    public Integer createNewAPI(Integer userId, Cred cred) {
        String auth = cred.getAuthorization();
        String writeAuth = "";
        // CRD
        if (auth == null || auth.equals(""))
            return -1;
        if (auth.contains("c") || auth.contains("C"))
            writeAuth += "C";
        if (auth.contains("r") || auth.contains("R"))
            writeAuth += "R";
        if (auth.contains("d") || auth.contains("D"))
            writeAuth += "D";
        if (writeAuth.equals(""))
            return -1;
        cred.setCredId(null);
        cred.setUserId(userId);
        cred.setToken(UUID.randomUUID().toString());
        cred.setAuthorization(writeAuth);
        return mapper.insert(cred);
***REMOVED***

    @Override
    public List<Cred> queryAPI(Integer userId) {
        QueryWrapper<Cred> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("user_id", userId);
        return mapper.selectList(queryWrapper);
***REMOVED***

    @Override
    public Integer updateAPI(Integer userId, Cred cred) {
        if (cred == null || cred.getCredId() == null || userId == null)
            return -1;
        QueryWrapper<Cred> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("cred_id", cred.getCredId());
        Cred cred1 = mapper.selectOne(queryWrapper);
        if (cred1 == null || !cred1.getUserId().equals(userId))
            return -1;
        String writeAuth = "";
        String auth = cred.getAuthorization();
        if (auth.contains("c") || auth.contains("C"))
            writeAuth += "C";
        if (auth.contains("r") || auth.contains("R"))
            writeAuth += "R";
        if (auth.contains("d") || auth.contains("D"))
            writeAuth += "D";
        if (writeAuth.equals(""))
            return -1;
        UpdateWrapper<Cred> updateWrapper = new UpdateWrapper<>();
        updateWrapper.eq("cred_id", cred.getCredId());
        updateWrapper.set("authorization", writeAuth);
        return mapper.update(null, updateWrapper);
***REMOVED***

    @Override
    public Integer deleteAPI(Integer userId, Integer credId) {
        if (credId == null || userId == null)
            return -1;
        QueryWrapper<Cred> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("cred_id", credId);
        Cred cred1 = mapper.selectOne(queryWrapper);
        if (cred1 == null || !cred1.getUserId().equals(userId))
            return -1;
        return mapper.deleteById(credId);
***REMOVED***

    @Override
    public Integer getUserId(String token) {
        QueryWrapper<Cred> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("token", token);
        Cred cred1 = mapper.selectOne(queryWrapper);
        return cred1 == null ? null : cred1.getUserId();
***REMOVED***

    @Override
    public String getAuth(String token) {
        QueryWrapper<Cred> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("token", token);
        Cred cred1 = mapper.selectOne(queryWrapper);
        return cred1 == null ? null : cred1.getAuthorization();
***REMOVED***

    @Override
    public byte[] render(Integer userId, String uuid) {
        QueryWrapper<Photo> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("photo_uuid", uuid);
        Photo photo = photoMapper.selectOne(queryWrapper);
        if (photo == null || !photo.getUserId().equals(userId))
            return new byte[0];
        String UUID = userService.getById(userId).getUuid();
        File file = new File(uploadFolder + System.getProperty("file.separator") + UUID + System.getProperty("file.separator") + uuid
                + "." + photo.getFormat());
        FileInputStream inputStream = null;
***REMOVED***
            inputStream = new FileInputStream(file);
            byte[] bytes = new byte[inputStream.available()];
            inputStream.read(bytes, 0, inputStream.available());
            return bytes;
***REMOVED*** catch (IOException e) {
            e.printStackTrace();
***REMOVED***
        return new byte[0];
***REMOVED***

    @Override
    public Integer deletePhoto(Integer userId, String uuid) {
        QueryWrapper<Photo> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("photo_uuid", uuid);
        Photo photo = photoMapper.selectOne(queryWrapper);
        if (photo == null || !photo.getUserId().equals(userId))
            return -1;
        return photoService.deletePhoto(userId, photo.getPhotoId());
***REMOVED***

    @Override
    public Namespace queryRootNamespace(Integer userId) {
        return namespaceService.queryRootNamespace(userId);
***REMOVED***

    @Override
    public List<Namespace> queryNamespaces(Integer userId, Integer parentId) {
        return namespaceService.queryNamespace(userId, parentId);
***REMOVED***

    @Override
    public List<Photo> queryPhotoList(Integer userId, Integer nsId) {
        QueryWrapper<Namespace> nsVerify = new QueryWrapper<>();
        nsVerify.eq("ns_id", nsId);
        nsVerify.eq("user_id", userId);
        Namespace namespace = namespaceService.getOne(nsVerify);
        if (namespace == null || !userId.equals(namespace.getUserId()))
    ***REMOVED***

        QueryWrapper<Photo> queryWrapper = new QueryWrapper<>();
        queryWrapper.eq("ns_id", nsId);
        return photoMapper.selectList(queryWrapper);
***REMOVED***


***REMOVED***
