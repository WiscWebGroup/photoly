package org.chengbing.service.impl;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import org.chengbing.entity.Gallery;
import org.chengbing.entity.Tag;
import org.chengbing.dao.TagMapper;
import org.chengbing.service.ITagService;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.util.List;
import java.util.Objects;

***REMOVED***
 * <p>
 *  服务实现类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
@Service
public class TagServiceImpl extends ServiceImpl<TagMapper, Tag> implements ITagService {

    @Resource
    TagMapper mapper;
    @Override
    public List<Tag> queryTag(Integer userId) {
        QueryWrapper<Tag> wrapper = new QueryWrapper<>();
        wrapper.eq("user_id", userId);
        return mapper.selectList(wrapper);
***REMOVED***

    @Override
    public Integer deleteTag(Integer userId, Integer tagId) {
        QueryWrapper<Tag> wrapper = new QueryWrapper<>();
        wrapper.eq("tag_id", tagId);
        wrapper.eq("user_id", userId);
        Tag selectTag = mapper.selectOne(wrapper);
        if (selectTag!=null && selectTag.getTagId() != null && selectTag.getUserId() != null && Objects.equals(selectTag.getUserId(), userId) && Objects.equals(selectTag.getTagId(), tagId))
            return mapper.deleteById(tagId);
        return -1;
***REMOVED***

    @Override
    public Integer updateTag(int userId, Tag tag) {
        tag.setUserId(userId);
        QueryWrapper<Tag> wrapper = new QueryWrapper<>();
        wrapper.eq("tag_id", tag.getTagId());
        wrapper.eq("user_id", userId);
        Tag selectTag = mapper.selectOne(wrapper);
        if (selectTag!=null && selectTag.getTagId() != null && selectTag.getUserId() != null && selectTag.getUserId() == userId && selectTag.getTagId() == tag.getTagId())
    ***REMOVED***
            UpdateWrapper<Tag> updateWrapper = new UpdateWrapper<>();
            updateWrapper.eq("tag_id", tag.getTagId());
            updateWrapper.set("tag_name", tag.getTagName());
            return mapper.update(null, updateWrapper);
***REMOVED***
        return -1;
***REMOVED***

    @Override
    public Integer insertTag(Integer userId, Tag tag) {
        if (tag.getTagName() == null || tag.getTagName().equals(""))
            return -1;
        tag.setUserId(userId);
        return mapper.insert(tag);
***REMOVED***
***REMOVED***
