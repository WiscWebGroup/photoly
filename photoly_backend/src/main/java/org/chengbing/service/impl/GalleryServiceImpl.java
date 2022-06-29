package org.chengbing.service.impl;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import com.baomidou.mybatisplus.core.metadata.IPage;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import org.chengbing.entity.Gallery;
import org.chengbing.dao.GalleryMapper;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import org.chengbing.service.IGalleryService;
import org.chengbing.util.ResultPage;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.time.LocalDateTime;
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
public class GalleryServiceImpl extends ServiceImpl<GalleryMapper, Gallery> implements IGalleryService {

    @Resource
    GalleryMapper mapper;
    @Override
    public Integer insertGallery(String name, Integer userId, String color) {
        Gallery gallery = new Gallery();
        gallery.setGaName(name);
        gallery.setCreateDate(LocalDateTime.now());
        gallery.setUserId(userId);
        gallery.setCoverColor(color);
        return mapper.insert(gallery);
***REMOVED***

    @Override
    public List<Gallery> queryGallery(Integer userId) {
        QueryWrapper<Gallery> wrapper = new QueryWrapper<>();
        wrapper.eq("user_id", userId);
        return mapper.selectList(wrapper);
***REMOVED***

    @Override
    public ResultPage<List<Gallery>> queryGalleryPage(Integer userId, Integer page, Integer rowsPerPage) {
        QueryWrapper<Gallery> wrapper = new QueryWrapper<>();
        wrapper.eq("user_id", userId);
        Page<Gallery> galleryPage = new Page<>(page, rowsPerPage);
        IPage<Gallery> galleries = mapper.selectPage(galleryPage, wrapper);
        return new ResultPage<>(galleries.getRecords(), Math.toIntExact(galleries.getPages()), 200);
***REMOVED***

    @Override
    public Integer deleteGallery(Integer userId, Integer gaId) {
        QueryWrapper<Gallery> wrapper = new QueryWrapper<>();
        wrapper.eq("ga_id", gaId);
        wrapper.eq("user_id", userId);
        Gallery gallery = mapper.selectOne(wrapper);
        if (Objects.equals(gallery.getGaId(), gaId) && Objects.equals(gallery.getUserId(), userId))
            return mapper.deleteById(gaId);
        return -1;
***REMOVED***

    @Override
    public Integer updateGallery(Integer userId, Integer gaId, Gallery gallery) {
        QueryWrapper<Gallery> wrapper = new QueryWrapper<>();
        wrapper.eq("ga_id", gaId);
        wrapper.eq("user_id", userId);
        Gallery selectGallery = mapper.selectOne(wrapper);
        if (Objects.equals(selectGallery.getGaId(), gaId) && Objects.equals(selectGallery.getUserId(), userId))
    ***REMOVED***
            UpdateWrapper<Gallery> updateWrapper = new UpdateWrapper<>();
            updateWrapper.eq("ga_id", gaId);
            updateWrapper.set("ga_name", gallery.getGaName());
            updateWrapper.set("cover_color", gallery.getCoverColor());
            updateWrapper.set("cover_id", gallery.getCoverId());
            return mapper.update(null, updateWrapper);
***REMOVED***
***REMOVED***
***REMOVED***
***REMOVED***
