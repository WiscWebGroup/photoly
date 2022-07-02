package org.chengbing.service.impl;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import org.chengbing.dao.NamespacePhotoMapper;
import org.chengbing.dao.PhotoMapper;
import org.chengbing.dao.UserMapper;
import org.chengbing.entity.Namespace;
import org.chengbing.dao.NamespaceMapper;
import org.chengbing.entity.NamespacePhoto;
import org.chengbing.entity.Photo;
import org.chengbing.entity.Tag;
import org.chengbing.service.INamespaceService;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.io.File;
import java.util.List;
import java.util.Objects;

/**
 * <p>
 *  服务实现类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@Service
public class NamespaceServiceImpl extends ServiceImpl<NamespaceMapper, Namespace> implements INamespaceService {

    @Resource
    NamespaceMapper mapper;

    @Resource
    NamespacePhotoMapper namespacePhotoMapper;

    @Resource
    PhotoMapper photoMapper;

    @Resource
    UserMapper userMapper;

    @Value("${file.uploadFolder}")
    String folderPath;

    @Override
    public Integer insertNamespace(String name, Integer parentId, Integer userId) {
        Namespace namespace = new Namespace();
        namespace.setNsName(name);
        namespace.setNsParentId(parentId);
        namespace.setUserId(userId);
        return mapper.insert(namespace);
    }

    @Override
    public Namespace queryRootNamespace(Integer userId) {
        QueryWrapper<Namespace> wrapper = new QueryWrapper<>();
        wrapper.eq("user_id", userId);
        wrapper.eq("ns_name", "/");
        return mapper.selectOne(wrapper);
    }

    @Override
    public List<Namespace> queryNamespace(Integer userId, Integer parentId) {
        QueryWrapper<Namespace> wrapper = new QueryWrapper<>();
        wrapper.eq("user_id", userId);
        wrapper.eq("ns_parent_id", parentId);
        return mapper.selectList(wrapper);
    }

    @Override
    public Integer updateNamespaceName(int userId, Namespace namespace) {
        if (namespace.getNsName() == null || namespace.getNsName().equals("") || namespace.getNsName().equals("/"))
            return -1;
        QueryWrapper<Namespace> wrapper = new QueryWrapper<>();
        wrapper.eq("ns_id", namespace.getNsId());
        wrapper.eq("user_id", userId);
        Namespace selected = mapper.selectOne(wrapper);
        if (selected!=null && selected.getNsId() != null && selected.getUserId() != null && selected.getUserId() == userId &&
                Objects.equals(selected.getNsId(), namespace.getNsId()) && !namespace.getNsName().equals("/") && !selected.getNsName().equals("/"))
        {
            UpdateWrapper<Namespace> updateWrapper = new UpdateWrapper<>();
            updateWrapper.eq("ns_id", namespace.getNsId());
            updateWrapper.set("ns_name", namespace.getNsName());
            return mapper.update(null, updateWrapper);
        }
        return -1;
    }

    @Override
    public Integer updateNamespaceParent(int userId, Namespace namespace) {
        if (namespace.getNsParentId() == null || namespace.getNsParentId() < 0)
            return -1;
        QueryWrapper<Namespace> wrapper = new QueryWrapper<>();
        wrapper.eq("ns_id", namespace.getNsId());
        wrapper.eq("user_id", userId);
        Namespace selected = mapper.selectOne(wrapper);
        if (selected!=null && selected.getNsId() != null && selected.getUserId() != null && selected.getUserId() == userId && Objects.equals(selected.getNsId(), namespace.getNsId())
        && !selected.getNsName().equals("/"))
        {
            UpdateWrapper<Namespace> updateWrapper = new UpdateWrapper<>();
            updateWrapper.eq("ns_id", namespace.getNsId());
            updateWrapper.set("ns_parent_id", namespace.getNsParentId());
            return mapper.update(null, updateWrapper);
        }
        return -1;
    }

    @Override
    public Integer insertNamespace(Integer userId, Namespace namespace) {
        if (namespace.getNsName() == null || namespace.getNsName().equals("") || namespace.getNsName().equals("/") || namespace.getNsParentId() == null)
            return -1;
        namespace.setUserId(userId);
        return mapper.insert(namespace);
    }

    @Override
    public Integer deleteNamespace(Integer userId, Integer nsId) {
        QueryWrapper<Namespace> wrapper = new QueryWrapper<>();
        wrapper.eq("ns_id", nsId);
        wrapper.eq("user_id", userId);
        Namespace selected = mapper.selectOne(wrapper);
        if (selected!=null && selected.getNsId() != null && selected.getUserId() != null && Objects.equals(selected.getUserId(), userId) &&
                Objects.equals(selected.getNsId(), nsId) && !selected.getNsName().equals("/"))
        {
            String userUUID = userMapper.selectById(userId).getUuid();
            // Delete all the namespaces whose parent folder is this one and delete all photos
            return deletePhotoHelper(userId, nsId, userUUID);
        }
        return -1;
    }

    public Integer deletePhotoHelper(Integer userId, Integer nsId, String userUUID)
    {
        int results = 0;
        // Get all sub namespaces
        QueryWrapper<Namespace> wrapper = new QueryWrapper<>();
        wrapper.eq("user_id", userId);
        wrapper.eq("ns_parent_id", nsId);
        List<Namespace> data = mapper.selectList(wrapper);
        // Get all photos in the current namespace
        QueryWrapper<Photo> wrapper2 = new QueryWrapper<>();
        wrapper2.eq("ns_id", nsId);
        List<Photo> connection = photoMapper.selectList(wrapper2);
        // Delete photos both in table record and in hard drive
        for(Photo currPhoto : connection)
        {
            int photoId = currPhoto.getPhotoId();
            String photoUUID = currPhoto.getPhotoUuid();
            String format = currPhoto.getFormat();
            // Delete photo in real place
            String photoPath = folderPath + System.getProperty("file.separator") + userUUID + System.getProperty("file.separator") + photoUUID + "." + format;
            File photo = new File(photoPath);
            results += photo.delete() ? 1 : 0;
            photoMapper.deleteById(photoId);
        }
        // Recursive on sub namespaces
        for(Namespace namespace : data)
            results += deletePhotoHelper(userId, namespace.getNsId(), userUUID);
        results += mapper.deleteById(nsId);
        return results;
    }
}
