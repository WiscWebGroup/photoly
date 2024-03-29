package org.chengbing.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import java.time.LocalDateTime;
import java.io.Serializable;
import io.swagger.annotations.ApiModel;
import lombok.Data;
import lombok.EqualsAndHashCode;

/**
 * <p>
 * 
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@Data
@EqualsAndHashCode(callSuper = false)
@ApiModel(value="Gallery对象", description="")
public class Gallery implements Serializable {

    private static final long serialVersionUID = 1L;

    @TableId(value = "ga_id", type = IdType.AUTO)
    private Integer gaId;

    private String gaName;

    private Integer userId;

    private LocalDateTime createDate;

    private Integer coverId;

    private String coverColor;


}
