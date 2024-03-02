package org.chengbing.service.impl;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import com.baomidou.mybatisplus.core.metadata.IPage;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import org.apache.commons.io.FileUtils;
import org.chengbing.entity.Gallery;
import org.chengbing.dao.GalleryMapper;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import org.chengbing.entity.Photo;
import org.chengbing.service.IGalleryService;
import org.chengbing.service.IPhotoService;
import org.chengbing.util.ResultPage;
import org.chengbing.util.ZipUtil;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.io.File;
import java.io.IOException;
import java.time.LocalDateTime;
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
public class GalleryServiceImpl extends ServiceImpl<GalleryMapper, Gallery> implements IGalleryService {

    @Resource
    GalleryMapper mapper;

    @Value("${file.uploadFolder}")
    String uploadFolder;

    @Resource
    IPhotoService photoService;


    @Override
    public Integer insertGallery(String name, Integer userId, String color) {
        Gallery gallery = new Gallery();
        if (name.equals("My Favorite"))
        {
            gallery.setCoverId(2);
        }
        gallery.setGaName(name);
        gallery.setCreateDate(LocalDateTime.now());
        gallery.setUserId(userId);
        gallery.setCoverColor(color);
        return mapper.insert(gallery);
    }

    @Override
    public List<Gallery> queryGallery(Integer userId) {
        QueryWrapper<Gallery> wrapper = new QueryWrapper<>();
        wrapper.eq("user_id", userId);
        return mapper.selectList(wrapper);
    }

    @Override
    public ResultPage<List<Gallery>> queryGalleryPage(Integer userId, Integer page, Integer rowsPerPage) {
        QueryWrapper<Gallery> wrapper = new QueryWrapper<>();
        wrapper.eq("user_id", userId);
        Page<Gallery> galleryPage = new Page<>(page, rowsPerPage);
        IPage<Gallery> galleries = mapper.selectPage(galleryPage, wrapper);
        return new ResultPage<>(galleries.getRecords(), Math.toIntExact(galleries.getPages()), 200);
    }

    @Override
    public Integer deleteGallery(Integer userId, Integer gaId) {
        QueryWrapper<Gallery> wrapper = new QueryWrapper<>();
        wrapper.eq("ga_id", gaId);
        wrapper.eq("user_id", userId);
        Gallery gallery = mapper.selectOne(wrapper);
        if (gallery != null && Objects.equals(gallery.getGaId(), gaId) && Objects.equals(gallery.getUserId(), userId))
            return mapper.deleteById(gaId);
        return -1;
    }

    @Override
    public Integer updateGallery(Integer userId, Integer gaId, Gallery gallery) {
        QueryWrapper<Gallery> wrapper = new QueryWrapper<>();
        wrapper.eq("ga_id", gaId);
        wrapper.eq("user_id", userId);
        Gallery selectGallery = mapper.selectOne(wrapper);
        if (selectGallery != null && Objects.equals(selectGallery.getGaId(), gaId) && Objects.equals(selectGallery.getUserId(), userId))
        {
            UpdateWrapper<Gallery> updateWrapper = new UpdateWrapper<>();
            updateWrapper.eq("ga_id", gaId);
            updateWrapper.set("ga_name", gallery.getGaName());
            updateWrapper.set("cover_color", gallery.getCoverColor());
            updateWrapper.set("cover_id", gallery.getCoverId());
            return mapper.update(null, updateWrapper);
        }
        return -1;
    }

    @Override
    public String downloadGallery(Integer userId, Integer gaId, String userUUID) {
        QueryWrapper<Gallery> wrapper = new QueryWrapper<>();
        wrapper.eq("ga_id", gaId);
        wrapper.eq("user_id", userId);
        Gallery selectGallery = mapper.selectOne(wrapper);
        if (selectGallery != null && Objects.equals(selectGallery.getGaId(), gaId) && Objects.equals(selectGallery.getUserId(), userId))
        {
            String tempFolderName = uploadFolder + File.separator + userUUID + File.separator + selectGallery.getGaName();
            String zipLoc = uploadFolder + File.separator + userUUID + File.separator + selectGallery.getGaName() + ".zip";

            File myGallery = new File(tempFolderName);
            if (myGallery.exists()) {
                try {
                    FileUtils.deleteDirectory(myGallery);
                } catch (IOException e) {
                    throw new RuntimeException(e);
                }
            }
            File lastZip = new File(zipLoc);
            if (lastZip.exists())
                lastZip.delete();
            myGallery.mkdirs();
            List<Photo> toTransferPhotos = photoService.queryPhotoByGallery(userId, gaId);
            for(Photo photo : toTransferPhotos)
            {
                String orgUrl = uploadFolder + File.separator + userUUID + File.separator + photo.getPhotoUuid() + "." + photo.getFormat();
                try {
                    FileUtils.copyFile(new File(orgUrl), new File(tempFolderName + File.separator + photo.getPhotoName() + "." + photo.getFormat()));
                } catch (IOException e) {
                    throw new RuntimeException(e);
                }
            }
            try {
                ZipUtil.compressZipFile(tempFolderName, zipLoc);
                myGallery = new File(tempFolderName);
                if (myGallery.exists()) {
                    try {
                        FileUtils.deleteDirectory(myGallery);
                    } catch (IOException e) {
                        throw new RuntimeException(e);
                    }
                }
                return zipLoc;
            } catch (IOException e) {
                throw new RuntimeException(e);
            }
        }
        return null;
    }
}
